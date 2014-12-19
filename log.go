// Copyright 2013, Örjan Persson. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logging

import (
	"bytes"
	"fmt"
	"io"
	"log"
)

// TODO initialize here
var colors []string
var boldcolors []string

type color int

const (
	colorBlack = (iota + 30)
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite
)

// LogBackend utilizes the standard log module.
type LogBackend struct {
	Logger *log.Logger
	Color  bool
}

// NewLogBackend creates a new LogBackend.
func NewLogBackend(out io.Writer, prefix string, flag int) *LogBackend {
	return &LogBackend{Logger: log.New(out, prefix, flag)}
}

func (b *LogBackend) Log(level Level, calldepth int, rec *Record) error {
	if b.Color {
		buf := &bytes.Buffer{}
		buf.Write([]byte(colors[level]))
		buf.Write([]byte(rec.Formatted(calldepth + 1)))
		buf.Write([]byte("\033[0m"))
		// For some reason, the Go logger arbitrarily decided "2" was
		// the correct call depth...
		return b.Logger.Output(calldepth+2, buf.String())
	} else {
		return b.Logger.Output(calldepth+2, rec.Formatted(calldepth+1))
	}
	panic("should not be reached")
}

func colorSeq(color color) string {
	return fmt.Sprintf("\033[%dm", int(color))
}

func colorSeqBold(color color) string {
	return fmt.Sprintf("\033[%d;1m", int(color))
}

func colorSeqBg(color color) string {
	return fmt.Sprintf("\033[%dm", int(10+color))
}

func colorSeqFaint(color color) string {
	return fmt.Sprintf("\033[%d;2m", int(color))
}

func init() {
	colors = []string{
		CRITICAL: colorSeq(colorMagenta),
		ERROR:    colorSeq(colorRed),
		WARNING:  colorSeq(colorYellow),
		NOTICE:   colorSeq(colorGreen),
		INFO:     colorSeqFaint(colorGreen),
		DEBUG:    colorSeqFaint(colorCyan),
	}
	boldcolors = []string{
		CRITICAL: colorSeqBold(colorMagenta),
		ERROR:    colorSeqBold(colorRed),
		WARNING:  colorSeqBold(colorYellow),
		NOTICE:   colorSeqBold(colorGreen),
		INFO:     colorSeq(colorGreen),
		DEBUG:    colorSeq(colorWhite),
	}
}
