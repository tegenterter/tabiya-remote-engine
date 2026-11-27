package main

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// A page's first line is the computer's cores, before anything the engine writes.
func TestRunSaysTheCoresFirst(t *testing.T) {
	eng := engine{path: script(t, `while read line; do [ "$line" = uci ] && echo uciok; [ "$line" = quit ] && exit 0; done`)}
	ts := httptest.NewServer(nil)
	defer ts.Close()
	port := ts.Listener.Addr().(*net.TCPAddr).Port
	ts.Config.Handler = &server{engine: eng, origins: []string{"http://localhost:5173"}, port: port}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/uci", port), &websocket.DialOptions{HTTPHeader: map[string][]string{"Origin": {"http://localhost:5173"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if err := c.Write(ctx, websocket.MessageText, []byte("uci")); err != nil {
		t.Fatal(err)
	}
	var got []string
	for len(got) < 2 {
		_, msg, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(msg))
	}
	want := []string{fmt.Sprintf("info string tabiya-remote-engine cores %d", runtime.NumCPU()), "uciok"}
	if got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %q, want %q", got, want)
	}
}
