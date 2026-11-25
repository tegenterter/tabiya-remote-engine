# Tabiya remote engine

This application runs any chess engine installed on your computer and makes it available to [Tabiya](https://tabiya.cloud) for analyzing positions or entire games, instead of running the engine in your browser. It works with any UCI-compatible engine (such as the popular Stockfish) and runs it natively, giving it the available cores and memory on your machine.

How to set it up in Tabiya: [tabiya.cloud/help/local-engine](https://tabiya.cloud/help/local-engine).

## Usage

Put the helper and your engine in the same folder on your computer, and open the helper:

```
my-folder/
  tabiya-remote-engine
  stockfish/
    stockfish-macos-m1-apple-silicon
```

Or give the engine's path, with its arguments after `--`:

```bash
tabiya-remote-engine /opt/homebrew/bin/stockfish
tabiya-remote-engine -- /usr/local/bin/lc0 --weights=/path/to/net.pb.gz
```

The helper checks that the engine answers as a UCI engine, then waits for Tabiya:

```
Stockfish 18 is ready for Tabiya on ws://127.0.0.1:9472/uci. Choose "Local engine" in Tabiya's engine settings; stop with Ctrl-C.
```

In Tabiya, choose **Local engine** in the engine's settings and turn the analysis on.

### Finding the engine

Without a path, the helper looks in its own folder, and in the folders inside it, for a program whose name starts with a known engine's name or with `engine`. That covers Stockfish's downloads, which unzip into a `stockfish` folder. The known names are `stockfish`, `lc0`, `reckless`, `berserk`, `obsidian`, `caissa`, `ethereal`, `koivisto`, `rubichess`, `viridithas`, `plentychess`, `alexandria`, `stormphrax`, `torch`, `dragon` and `komodo`. It never starts any other program in the folder. If it finds no engine, or more than one, it says so and waits for Enter, so a double-clicked window stays open.

### Flags

| Flag | Default | |
| --- | --- | --- |
| `-port` | `9472` | The port on 127.0.0.1. Tabiya expects 9472. |
| `-origin` | | Another site that may connect, such as a staging site. Repeat the flag for more than one. |

## How it works

The helper listens on `127.0.0.1:9472`, so it's reachable from your computer only. Each Tabiya page that connects opens a WebSocket at `/uci`. The helper then starts the engine for that page and passes UCI lines both ways, one message per line. When the page leaves, it tells the engine to quit, and stops it a second later if it hasn't.

### Security

- **Only Tabiya's pages can connect.** Their `Origin` must be `https://tabiya.cloud`, or Tabiya's development servers (`http://localhost:5173` and `:4173`), or a site given with `-origin`.
- **Only from your computer.** The `Host` must be the loopback address, so a site that points its own name at 127.0.0.1 (DNS rebinding) is refused.
- **Only the engine you chose runs.** A page can't choose a program, a path or the engine's arguments.
- **Only the commands Tabiya analyses with reach the engine:** `uci`, `isready`, `ucinewgame`, `position`, `go`, `stop` and `ponderhit`. Of the options, only `Threads`, `Hash` and `MultiPV` can be set, and only to numbers. No option that names a file gets through, such as Stockfish's `Debug Log File`, which would let a page write to your disk.
- **Your positions never leave your computer.** The helper doesn't contact Tabiya's servers or anything else on the internet.

## Building

```bash
go build
go test ./...
```

It needs Go 1.27 or later. Its only dependency is [`github.com/coder/websocket`](https://github.com/coder/websocket).
