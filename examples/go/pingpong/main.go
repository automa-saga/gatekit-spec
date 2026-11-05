package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:9099", "TCP address (host:port)")
	upPing := fs.Int("upgrade-ping", 5, "message at which ping is upgraded to the new binary")
	upPong := fs.Int("upgrade-pong", 3, "message at which pong is upgraded to the new binary")
	lead := fs.Int("lead", 2, "once both are upgraded, propose the v2 flip this many messages ahead")
	rounds := fs.Int("rounds", 12, "stop after this many messages")
	delay := fs.Duration("delay", 400*time.Millisecond, "pause before each reply")
	_ = fs.Parse(os.Args[2:])

	cfg := Config{UpgradePing: *upPing, UpgradePong: *upPong, Lead: *lead, Total: *rounds, Delay: *delay}
	// Validate the pacing flags. A Lead < 1 would make agreed <= seq at proposal time, so
	// the proposal message itself would be sent as v2 — breaking the "propose in a v1
	// message" invariant. The others must be positive message numbers.
	if cfg.Lead < 1 || cfg.Total < 1 || cfg.UpgradePing < 1 || cfg.UpgradePong < 1 {
		fmt.Fprintln(os.Stderr, "pingpong: -lead, -rounds, -upgrade-ping, -upgrade-pong must all be >= 1")
		os.Exit(2)
	}
	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }

	var err error
	switch cmd {
	case "pong":
		err = serve(*addr, cfg, logf)
	case "ping":
		err = dial(*addr, cfg, logf)
	case "demo":
		err = runDemo(cfg, logf)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pingpong:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `pingpong — a rolling wire-format upgrade across two peers

Start pong first, then ping in a second terminal. Each peer starts on the old binary
(v1 only), is upgraded mid-conversation to the new binary (reads v1 and v2, still sends
v1), and once both are upgraded they flip to sending v2 at an agreed message number.

usage:
  pingpong pong [flags]    # start first: listens, plays "pong"
  pingpong ping [flags]    # then this, in another terminal: connects, plays "ping"
  pingpong demo [flags]    # run both in one process (what the test drives)

flags:
  -addr host:port     TCP address (default 127.0.0.1:9099)
  -upgrade-ping N     message at which ping is upgraded (default 5)
  -upgrade-pong N     message at which pong is upgraded (default 3)
  -lead N             propose the v2 flip N messages ahead, once both upgraded (default 2)
  -rounds N           stop after N messages (default 12)
  -delay D            pause before each reply (default 400ms)
`)
}
