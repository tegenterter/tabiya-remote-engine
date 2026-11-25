package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func script(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts stand in for engines")
	}
	p := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProbe(t *testing.T) {
	name, err := engine{path: script(t, `read line; echo "id name Fake 1"; echo uciok`)}.probe()
	if err != nil || name != "Fake 1" {
		t.Errorf("a UCI engine: %q, %v", name, err)
	}
	if _, err := (engine{path: script(t, `exit 1`)}).probe(); !errors.Is(err, errSilent) {
		t.Errorf("a silent program: %v, want errSilent", err)
	}
	if _, err := (engine{path: script(t, `echo "usage: something"`)}).probe(); err == nil || errors.Is(err, errSilent) {
		t.Errorf("another program: %v, want another error", err)
	}
}
