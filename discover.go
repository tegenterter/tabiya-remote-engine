package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var engineNames = []string{
	"stockfish", "lc0", "reckless", "berserk", "obsidian", "caissa", "ethereal", "koivisto", "rubichess",
	"viridithas", "plentychess", "alexandria", "stormphrax", "torch", "dragon", "komodo", "engine",
}

func discover(dir, self string) (string, error) {
	var found []string
	for _, pattern := range []string{"*", filepath.Join("*", "*")} {
		paths, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return "", err
		}
		for _, p := range paths {
			if p != self && looksLikeEngine(p) {
				found = append(found, p)
			}
		}
	}
	sort.Strings(found)
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no engine in %s: put one there (Stockfish, say, from https://stockfishchess.org/download/), or give its path: tabiya-remote-engine /path/to/engine", dir)
	case 1:
		return found[0], nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "more than one engine in %s; keep one, or give its path: tabiya-remote-engine /path/to/engine", dir)
	for _, p := range found {
		rel, _ := filepath.Rel(dir, p)
		fmt.Fprintf(&b, "\n  %s", rel)
	}
	return "", errors.New(b.String())
}

func looksLikeEngine(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	name := strings.ToLower(filepath.Base(path))
	if runtime.GOOS == "windows" {
		var ok bool
		if name, ok = strings.CutSuffix(name, ".exe"); !ok {
			return false
		}
	} else if info.Mode().Perm()&0o111 == 0 {
		return false
	}

	switch filepath.Ext(name) {
	case ".nnue", ".pb", ".gz", ".zip", ".tar", ".dylib", ".so", ".dll", ".txt", ".md":
		return false
	}
	for _, n := range engineNames {
		if strings.HasPrefix(name, n) {
			return true
		}
	}
	return false
}
