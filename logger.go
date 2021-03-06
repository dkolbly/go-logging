// Copyright 2013, Örjan Persson. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package logging implements a logging infrastructure for Go. It supports
// different logging backends like syslog, file and memory. Multiple backends
// can be utilized with different log levels per backend and logger.
package logging

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Redactor is an interface for types that may contain sensitive information
// (like passwords), which shouldn't be printed to the log. The idea was found
// in relog as part of the vitness project.
type Redactor interface {
	Redacted() interface{}
}

// Redact returns a string of * having the same length as s.
func Redact(s string) string {
	return strings.Repeat("*", len(s))
}

var (
	// Sequence number is incremented and utilized for all log records created.
	sequenceNo uint64

	// timeNow is a customizable for testing purposes.
	timeNow = time.Now
)

// Annotation is a key/value entry; the Annotator backend can
// add annotations
type Annotation struct {
	Key string
	Value interface{}
}

// Record represents a log record and contains the timestamp when the record
// was created, an increasing id, filename and line and finally the actual
// formatted log line.
type Record struct {
	Id     uint64
	Time   time.Time
	Module string
	Level  Level
	Annotations []Annotation

	// message is kept as a pointer to have shallow copies update this once
	// needed.
	message   *string
	args      []interface{}
	fmt       string
	formatter Formatter
	formatted string
}

func (r *Record) Formatted(calldepth int) string {
	if r.formatted == "" {
		var buf bytes.Buffer
		r.formatter.Format(calldepth+1, r, &buf)
		r.formatted = buf.String()
	}
	return r.formatted
}

func (r *Record) Message() string {
	if r.message == nil {
		// Redact the arguments that implements the Redactor interface
		for i, arg := range r.args {
			if redactor, ok := arg.(Redactor); ok == true {
				r.args[i] = redactor.Redacted()
			}
		}
		msg := fmt.Sprintf(r.fmt, r.args...)
		r.message = &msg
	}
	return *r.message
}

type LevelLogger interface {
	// a logging.LevelLogger is also a standardlog.Logger
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})

	// Go1 details of log.Logger that don't seem appropriate to
	// the abstract notion of logging
	Output(calldepth int, s string) error
	Outputf(adjcalldepth int, format string, args ...interface{})
	Prefix() string
	Flags() int
	SetFlags(flag int)
	SetPrefix(prefix string)

	// and then we have our own goodies

	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Notice(format string, args ...interface{})
	Warning(format string, args ...interface{})
	Error(format string, args ...interface{})
	Critical(format string, args ...interface{})
}

type Logger struct {
	Module string
	flags  int
	// the Level at which we Output records (defaults to INFO)
	OutputLevel Level
	backend LeveledBackend
	annotater Annotater
}

func (l *Logger) SetBackend(b LeveledBackend) {
	l.backend = b
}

func (l *Logger) Backend() LeveledBackend {
	if l.backend == nil {
		return defaultBackend
	} else {
		return l.backend
	}
}

// TODO call NewLogger and remove MustGetLogger?
// GetLogger creates and returns a Logger object based on the module name.
func GetLogger(module string) (*Logger, error) {
	return &Logger{
		Module:      module,
		OutputLevel: INFO,
	}, nil
}

// MustGetLogger is like GetLogger but panics if the logger can't be created.
// It simplifies safe initialization of a global logger for eg. a package.
func MustGetLogger(module string) *Logger {
	logger, err := GetLogger(module)
	if err != nil {
		panic("logger: " + module + ": " + err.Error())
	}
	return logger
}

// Reset restores the internal state of the logging library.
func Reset() {
	// TODO make a global Init() method to be less magic? or make it such that
	// if there's no backends at all configured, we could use some tricks to
	// automatically setup backends based if we have a TTY or not.
	sequenceNo = 0
	b := SetBackend(NewLogBackend(os.Stderr, "", log.LstdFlags))
	b.SetLevel(DEBUG, "")
	SetFormatter(DefaultFormatter)
	timeNow = time.Now
}

// InitForTesting is a convenient method when using logging in a test. Once
// called, the time will be frozen to January 1, 1970 UTC.
func InitForTesting(level Level) *MemoryBackend {
	Reset()

	memoryBackend := NewMemoryBackend(10240)

	leveledBackend := AddModuleLevel(memoryBackend)
	leveledBackend.SetLevel(level, "")
	SetBackend(leveledBackend)

	timeNow = func() time.Time {
		return time.Unix(0, 0).UTC()
	}
	return memoryBackend
}

// IsEnabledFor returns true if the logger is enabled for the given level.
func (l *Logger) IsEnabledFor(level Level) bool {
	return l.Backend().IsEnabledFor(level, l.Module)
}

func (l *Logger) Log(lvl Level, format string, args ...interface{}) {
	// Create the logging record and pass it in to the backend
	record := &Record{
		Id:     atomic.AddUint64(&sequenceNo, 1),
		Time:   timeNow(),
		Module: l.Module,
		Level:  lvl,
		fmt:    format,
		args:   args,
	}
	if l.annotater != nil {
		l.annotater.Annotate(record)
	}
	// TODO use channels to fan out the records to all backends?
	// TODO in case of errors, do something (tricky)

	// calldepth=2 brings the stack up to the caller of the level
	// methods, Info(), Fatal(), etc.
	l.Backend().Log(lvl, 2, record)
}

func (l *Logger) Print(v ...interface{}) {
	l.Output(2, fmt.Sprint(v...))
}

func (l *Logger) Printf(format string, v ...interface{}) {
	l.Output(2, fmt.Sprintf(format, v...))
}

func (l *Logger) Println(v ...interface{}) {
	l.Output(2, fmt.Sprintln(v...))
}

func (l *Logger) Output(calldepth int, s string) error {
	// Create the logging record and pass it in to the backend
	record := &Record{
		Id:     atomic.AddUint64(&sequenceNo, 1),
		Time:   timeNow(),
		Module: l.Module,
		Level:  l.OutputLevel,
		fmt:    "%s",
		args:   []interface{}{s},
	}
	if l.annotater != nil {
		l.annotater.Annotate(record)
	}

	// TODO use channels to fan out the records to all backends?
	// TODO in case of errors, do something (tricky)

	// calldepth=2 brings the stack up to the caller of the level
	// methods, Info(), Fatal(), etc.
	l.Backend().Log(l.OutputLevel, calldepth, record)
	return nil
}


func (l *Logger) Outputf(adjdepth int, fmt string, args ...interface{}) {
	// Create the logging record and pass it in to the backend
	record := &Record{
		Id:     atomic.AddUint64(&sequenceNo, 1),
		Time:   timeNow(),
		Module: l.Module,
		Level:  l.OutputLevel,
		fmt:    fmt,
		args:   args,
	}

	// TODO use channels to fan out the records to all backends?
	// TODO in case of errors, do something (tricky)

	// calldepth=2 brings the stack up to the level of our caller
	l.Backend().Log(l.OutputLevel, 1+adjdepth, record)
}

func (l *Logger) SetFlags(flag int) {
	l.flags = flag
}

func (l *Logger) Flags() int {
	return l.flags
}

func (l *Logger) SetPrefix(p string) {
	l.Module = p
}

func (l *Logger) Prefix() string {
	return l.Module
}

// Fatal is equivalent to l.Critical(fmt.Sprint()) followed by a call to os.Exit(1).
func (l *Logger) Fatal(args ...interface{}) {
	s := fmt.Sprint(args...)
	l.Log(CRITICAL, "%s", s)
	os.Exit(1)
}

// Fatalf is equivalent to l.Critical followed by a call to os.Exit(1).
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Log(CRITICAL, format, args...)
	os.Exit(1)
}

// Fatalln is equivalent to l.Critical(fmt.Sprintln()) followed by a call to os.Exit(1).
func (l *Logger) Fatalln(args ...interface{}) {
	s := fmt.Sprintln(args...)
	l.Log(CRITICAL, "%s", s)
	os.Exit(1)
}

// Panic is equivalent to l.Critical(fmt.Sprint()) followed by a call to panic().
func (l *Logger) Panic(args ...interface{}) {
	s := fmt.Sprint(args...)
	l.Log(CRITICAL, "%s", s)
	panic(s)
}

// Panicf is equivalent to l.Critical followed by a call to panic().
func (l *Logger) Panicf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	l.Log(CRITICAL, "%s", s)
	panic(s)
}

// Panicln is equivalent to l.Critical(fmt.Sprintln()) followed by a call to panic().
func (l *Logger) Panicln(args ...interface{}) {
	s := fmt.Sprintln(args...)
	l.Log(CRITICAL, "%s", s)
	panic(s)
}

// Critical logs a message using CRITICAL as log level.
func (l *Logger) Critical(format string, args ...interface{}) {
	l.Log(CRITICAL, format, args...)
}

// Error logs a message using ERROR as log level.
func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(ERROR, format, args...)
}

// Warning logs a message using WARNING as log level.
func (l *Logger) Warning(format string, args ...interface{}) {
	l.Log(WARNING, format, args...)
}

// Notice logs a message using NOTICE as log level.
func (l *Logger) Notice(format string, args ...interface{}) {
	l.Log(NOTICE, format, args...)
}

// Info logs a message using INFO as log level.
func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(INFO, format, args...)
}

// Debug logs a message using DEBUG as log level.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(DEBUG, format, args...)
}

func init() {
	Reset()
}
