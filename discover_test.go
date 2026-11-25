package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func folder(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		mode := os.FileMode(0o644)
		if name, ok := strings.CutSuffix(f, "*"); ok {
			f, mode = name, 0o755
		}
		if runtime.GOOS == "windows" && mode == 0o755 {
			f += ".exe"
		}
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDiscover(t *testing.T) {
	for _, c := range []struct {
		name  string
		files []string
		want  string
	}{
		{"beside", []string{"tabiya-remote-engine*", "stockfish*"}, "stockfish"},
		{"in the download's folder", []string{"tabiya-remote-engine*", "stockfish/stockfish-macos-m1-apple-silicon*", "stockfish/README.md", "stockfish/AUTHORS"}, "stockfish/stockfish-macos-m1-apple-silicon"},
		{"any engine, renamed", []string{"tabiya-remote-engine*", "Engine-mine*"}, "Engine-mine"},
		{"a version in the name", []string{"tabiya-remote-engine*", "berserk-13.0-avx2*"}, "berserk-13.0-avx2"},
		{"other programs aren't engines", []string{"tabiya-remote-engine*", "installer*", "stockfish*"}, "stockfish"},
		{"not executable", []string{"tabiya-remote-engine*", "stockfish"}, ""},
		{"a network isn't an engine", []string{"tabiya-remote-engine*", "lc0*", "lc0-net.pb.gz*"}, "lc0"},
		{"none", []string{"tabiya-remote-engine*", "notes.txt"}, ""},
		{"two", []string{"tabiya-remote-engine*", "stockfish*", "lc0*"}, ""},
		{"not two folders deep", []string{"tabiya-remote-engine*", "a/b/stockfish*"}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := folder(t, c.files...)
			self := filepath.Join(dir, "tabiya-remote-engine")
			if runtime.GOOS == "windows" {
				self += ".exe"
			}
			got, err := discover(dir, self)
			if c.want == "" {
				if err == nil {
					t.Fatalf("found %s, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(dir, filepath.FromSlash(c.want))
			if runtime.GOOS == "windows" {
				want += ".exe"
			}
			if got != want {
				t.Errorf("found %s, want %s", got, want)
			}
		})
	}
}

func TestDiscoverNamesTheEngines(t *testing.T) {
	dir := folder(t, "stockfish*", "lc0*")
	_, err := discover(dir, filepath.Join(dir, "tabiya-remote-engine"))
	if err == nil || !strings.Contains(err.Error(), "stockfish") || !strings.Contains(err.Error(), "lc0") {
		t.Errorf("error %v, want both engines named", err)
	}
}
