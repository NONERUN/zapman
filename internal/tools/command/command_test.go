package command

import (
	"slices"
	"strings"
	"testing"
)

func TestLines(t *testing.T) {
	r := Result{Output: "  one \r\n\r\ntwo\n   \nthree"}
	if got := r.Lines(); !slices.Equal(got, []string{"one", "two", "three"}) {
		t.Errorf("Lines = %q", got)
	}
	if got := (Result{}).Lines(); got != nil {
		t.Errorf("empty Lines = %q", got)
	}
}

func TestSystem32(t *testing.T) {
	t.Setenv("SystemRoot", `D:\Win`)
	if got := System32("netsh.exe"); !strings.HasSuffix(got, "netsh.exe") || !strings.HasPrefix(got, `D:\Win`) {
		t.Errorf("System32 = %q", got)
	}
}
