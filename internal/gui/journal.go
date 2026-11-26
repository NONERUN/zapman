package gui

import (
	"strings"
	"time"
)

// journalMax is the number of lines that the journal keeps, as in the PS
// GUI.
const journalMax = 80

// journal is the action journal of the Status dialog. It is not safe for
// concurrent use.
type journal struct {
	lines []string
}

// add puts "HH:mm:ss  msg" at the end and removes the oldest lines.
func (j *journal) add(at time.Time, msg string) {
	j.lines = append(j.lines, at.Format("15:04:05")+"  "+msg)
	if n := len(j.lines) - journalMax; n > 0 {
		j.lines = append(j.lines[:0:0], j.lines[n:]...)
	}
}

func (j *journal) text() string { return strings.Join(j.lines, "\n") }
