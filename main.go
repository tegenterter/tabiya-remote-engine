package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const Port = 9472

var defaultOrigins = []string{
	"https://tabiya.cloud",
	"http://localhost:5173",
	"http://localhost:4173",
}

type origins []string

func (o *origins) String() string     { return strings.Join(*o, ",") }
func (o *origins) Set(s string) error { *o = append(*o, strings.TrimRight(s, "/")); return nil }

func main() {
	port := flag.Int("port", Port, "the port to listen on, on 127.0.0.1; the web app expects 9472")
	var extra origins
	flag.Var(&extra, "origin", "another site that may connect, e.g. https://staging.example.com (repeatable)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: tabiya-remote-engine [flags] [--] [/path/to/engine [engine arguments]]\n\nRuns a UCI chess engine for Tabiya's analysis: the one given, or else the one in the helper's folder.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	var eng engine
	if flag.NArg() > 0 {
		eng = engine{path: flag.Arg(0), args: flag.Args()[1:]}
	} else {
		fatal = func(format string, v ...any) {
			log.Printf(format, v...)
			fmt.Fprint(os.Stderr, "Press Enter to close.")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			os.Exit(1)
		}
		self, err := os.Executable()
		if err == nil {
			self, err = filepath.EvalSymlinks(self)
		}
		if err != nil {
			fatal("Can't tell where the helper is, to look for an engine beside it: %v", err)
		}
		path, err := discover(filepath.Dir(self), self)
		if err != nil {
			fatal("Found no engine to run: %v", err)
		}
		log.Printf("Found %s beside the helper.", path)
		eng = engine{path: path}
	}

	name, err := eng.probe()
	if errors.Is(err, errSilent) && runtime.GOOS == "darwin" {
		fatal("%s didn't start. macOS may have blocked it, as it does a downloaded program it can't check: open System Settings, then Privacy & Security, choose Open Anyway next to %q, then open the helper again.", eng.path, filepath.Base(eng.path))
	}
	if err != nil {
		fatal("%s doesn't answer as a UCI engine: %v", eng.path, err)
	}

	s := &server{engine: eng, origins: append(append([]string{}, defaultOrigins...), extra...), port: *port}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fatal("Can't listen on %s (is it running already?): %v", addr, err)
	}
	log.Printf("%s is ready for Tabiya on ws://%s/uci. Choose \"Local engine\" in Tabiya's engine settings; stop with Ctrl-C.", name, addr)
	log.Fatal(http.Serve(ln, s))
}

var fatal = log.Fatalf

type engine struct {
	path string
	args []string
}

var errSilent = errors.New("it wrote nothing")

func (e engine) probe() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.path, e.args...)
	cmd.Stdin = strings.NewReader("uci\n")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	name := e.path
	wrote := false
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		wrote = true
		line := strings.TrimSpace(sc.Text())
		if n, ok := strings.CutPrefix(line, "id name "); ok {
			name = n
		}
		if line == "uciok" {
			return name, nil
		}
	}
	if !wrote {
		return "", errSilent
	}
	if ctx.Err() != nil {
		return "", errors.New("no uciok within 10 s")
	}
	return "", errors.New("it ended without uciok")
}

type server struct {
	engine  engine
	origins []string
	port    int
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/uci" {
		http.NotFound(w, r)
		return
	}
	if !s.localHost(r.Host) {
		http.Error(w, "forbidden host", http.StatusForbidden)
		return
	}
	origin := r.Header.Get("Origin")
	if !s.allowedOrigin(origin) {
		log.Printf("Refused a connection from %q: not a Tabiya page (see -origin).", origin)
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 16)
	log.Printf("Analysing for %s.", origin)
	reason := s.run(r.Context(), c)
	log.Printf("Done for %s: %s.", origin, reason)
}

func (s *server) localHost(host string) bool {
	p := strconv.Itoa(s.port)
	return host == "127.0.0.1:"+p || host == "localhost:"+p || host == "[::1]:"+p
}

func (s *server) allowedOrigin(origin string) bool {
	for _, o := range s.origins {
		if origin == o {
			return true
		}
	}
	return false
}

func (s *server) run(ctx context.Context, c *websocket.Conn) string {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.Command(s.engine.path, s.engine.args...)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		c.Close(websocket.StatusInternalError, "could not start the engine")
		return err.Error()
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		c.Close(websocket.StatusInternalError, "could not start the engine")
		return err.Error()
	}
	if err := cmd.Start(); err != nil {
		c.Close(websocket.StatusInternalError, "could not start the engine")
		return err.Error()
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_, _ = io.WriteString(in, "quit\n")
			_ = in.Close()
			done := make(chan struct{})
			go func() {
				_ = cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		})
	}
	defer stop()

	ended := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 1<<16), 1<<20)
		for sc.Scan() {
			if err := c.Write(ctx, websocket.MessageText, sc.Bytes()); err != nil {
				ended <- nil
				return
			}
		}
		ended <- errors.New("the engine stopped")
		c.Close(websocket.StatusInternalError, "the engine stopped")
	}()

	for {
		_, msg, err := c.Read(ctx)
		if err != nil {
			select {
			case e := <-ended:
				if e != nil {
					return e.Error()
				}
			default:
			}
			return "the page left"
		}
		for _, line := range strings.Split(string(msg), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !allowed(line) {
				log.Printf("Not passed on to the engine: %q", line)
				continue
			}
			if _, err := io.WriteString(in, line+"\n"); err != nil {
				c.Close(websocket.StatusInternalError, "the engine stopped")
				return "the engine stopped"
			}
		}
	}
}

var options = map[string]bool{"Threads": true, "Hash": true, "MultiPV": true}

func allowed(line string) bool {
	for _, r := range line {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	f := strings.Fields(line)
	switch f[0] {
	case "uci", "isready", "ucinewgame", "stop", "ponderhit":
		return len(f) == 1
	case "position":
		for _, t := range f[1:] {
			for _, r := range t {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '/' || r == '-') {
					return false
				}
			}
		}
		return len(f) >= 2 && (f[1] == "startpos" || f[1] == "fen")
	case "go":
		for _, t := range f[1:] {
			if _, err := strconv.Atoi(t); err == nil {
				continue
			}
			switch t {
			case "depth", "movetime", "nodes", "mate", "infinite", "wtime", "btime", "winc", "binc", "movestogo", "ponder":
			default:
				return false
			}
		}
		return true
	case "setoption":
		if len(f) != 5 || f[1] != "name" || f[3] != "value" || !options[f[2]] {
			return false
		}
		_, err := strconv.Atoi(f[4])
		return err == nil
	}
	return false
}
