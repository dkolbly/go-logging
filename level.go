// Copyright 2013, Örjan Persson. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logging

import (
	"errors"
	"strings"
	"sync"
	"path"
)

var ErrInvalidLogLevel = errors.New("logger: invalid log level")

// Level defines all available log levels for log messages.
type Level int

const (
	CRITICAL Level = iota
	ERROR
	WARNING
	NOTICE
	INFO
	DEBUG
)

var levelNames = []string{
	"CRITICAL",
	"ERROR",
	"WARNING",
	"NOTICE",
	"INFO",
	"DEBUG",
}

// String returns the string representation of a logging level.
func (p Level) String() string {
	return levelNames[p]
}

// LogLevel returns the log level from a string representation.
func LogLevel(level string) (Level, error) {
	for i, name := range levelNames {
		if strings.EqualFold(name, level) {
			return Level(i), nil
		}
	}
	return ERROR, ErrInvalidLogLevel
}

type Leveled interface {
	GetLevel(string) Level
	SetLevel(Level, string)
	IsEnabledFor(Level, string) bool
}

// LeveledBackend is a log backend with additional knobs for setting levels on
// individual modules to different levels.
type LeveledBackend interface {
	Backend
	Leveled
}

type levelRule struct {
	pattern		string
	level		Level
}

type moduleLeveled struct {
	levels		map[string]Level
	exactMatch	map[string]Level
	patternRules	[]levelRule
	backend		Backend
	formatter	Formatter
	once		sync.Once
}

// AddModuleLevel wraps a log backend with knobs to have different log levels
// for different modules.
func AddModuleLevel(backend Backend) LeveledBackend {
	var leveled LeveledBackend
	var ok bool
	if leveled, ok = backend.(LeveledBackend); !ok {
		leveled = &moduleLeveled{
			levels:  make(map[string]Level),
			exactMatch: make(map[string]Level),
			patternRules: make([]levelRule, 0),
			backend: backend,
		}
	}
	return leveled
}

// GetLevel returns the log level for the given module.
func (l *moduleLeveled) GetLevel(module string) Level {
	level, exists := l.levels[module]
	if !exists {
		level, exists = l.exactMatch[module]
		if !exists {
			level = DEBUG // default value in case of no match
			for _, r := range l.patternRules {
				match, _ := path.Match(r.pattern, module)
				if match {
					level = r.level
				}
			}
		}
		// cache the result
		l.levels[module] = level
	}
	return level
}

// SetLevel sets the log level for the given module.  If the
// module contains one of the special characters '*' or '?',
// then it is interpreted as a glob (unless `path.Match` rejects
// the pattern as being malformed, in which case an error is logged
// and the module is considered an exact match)
func (l *moduleLeveled) SetLevel(level Level, module string) {
	// check for globbing
	isglob := false
	if strings.Contains(module, "*") || strings.Contains(module, "?") {
		_, err := path.Match(module, "testvalue")
		if err == nil {
			isglob = true
		} else {
			MustGetLogger("logger").Error("Invalid module pattern %q", module)
		}
	}
	// add the rule
	if isglob {
		l.patternRules = append(l.patternRules, levelRule{module, level})
		// clear the cache if there's anything in it
		if len(l.levels) > 0 {
			l.levels = make(map[string]Level)
		}
	} else {
		l.exactMatch[module] = level
		l.levels[module] = level
	}
}

// IsEnabledFor will return true if logging is enabled for the given module.
func (l *moduleLeveled) IsEnabledFor(level Level, module string) bool {
	return level <= l.GetLevel(module)
}

func (l *moduleLeveled) Log(level Level, calldepth int, rec *Record) (err error) {
	if l.IsEnabledFor(level, rec.Module) {
		rec.formatter = l.getFormatterAndCacheCurrent()
		err = l.backend.Log(level, calldepth+1, rec)
	}
	return
}

func (l *moduleLeveled) getFormatterAndCacheCurrent() Formatter {
	l.once.Do(func() {
		if l.formatter == nil {
			l.formatter = getFormatter()
		}
	})
	return l.formatter
}
