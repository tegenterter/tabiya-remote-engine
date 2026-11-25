package main

import "testing"

func TestAllowed(t *testing.T) {
	for line, want := range map[string]bool{
		"uci":                               true,
		"isready":                           true,
		"ucinewgame":                        true,
		"stop":                              true,
		"position startpos":                 true,
		"position startpos moves e2e4 e7e5": true,
		"position fen rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1 moves e7e5 g1f3": true,
		"position fen 8/8/8/8/8/8/8/K6k w - - 0 1 moves a7a8q":                                    true,
		"go depth 26":                    true,
		"go depth 18 movetime 2000":      true,
		"go infinite":                    true,
		"setoption name Threads value 8": true,
		"setoption name Hash value 512":  true,
		"setoption name MultiPV value 3": true,
		"":                               false,
		"uci extra":                      false,
		"quit":                           false,
		"debug on":                       false,
		"setoption name Debug Log File value /tmp/x": false,
		"setoption name SyzygyPath value /":          false,
		"setoption name EvalFile value nn.nnue":      false,
		"setoption name Hash value big":              false,
		"setoption name Threads":                     false,
		"go searchmoves e2e4":                        false,
		"position fen ../../etc":                     false,
		"position file x":                            false,
		"isready\x1b[2J":                             false,
	} {
		if line == "" {
			continue
		}
		if got := allowed(line); got != want {
			t.Errorf("allowed(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestHostAndOrigin(t *testing.T) {
	s := &server{port: 9472, origins: defaultOrigins}
	for host, want := range map[string]bool{"127.0.0.1:9472": true, "localhost:9472": true, "[::1]:9472": true, "evil.example:9472": false, "127.0.0.1:80": false} {
		if got := s.localHost(host); got != want {
			t.Errorf("localHost(%q) = %v, want %v", host, got, want)
		}
	}
	for origin, want := range map[string]bool{"https://tabiya.cloud": true, "http://localhost:5173": true, "http://tabiya.cloud": false, "https://tabiya.cloud.evil.example": false, "": false, "null": false} {
		if got := s.allowedOrigin(origin); got != want {
			t.Errorf("allowedOrigin(%q) = %v, want %v", origin, got, want)
		}
	}
}
