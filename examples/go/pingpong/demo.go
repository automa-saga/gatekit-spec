package main

import (
	"net"
	"sync"
)

// runDemo plays ping and pong in a single process, connected by an in-memory pipe
// instead of a real socket. It runs the exact same runPeer code the two-terminal
// commands use, so the demo is faithful — only the transport differs. main uses it for a
// one-command run; the test uses it for a deterministic check.
func runDemo(cfg Config, logf func(string, ...any)) error {
	c1, c2 := net.Pipe()
	tag := func(name string) func(string, ...any) {
		return func(format string, args ...any) { logf("["+name+"] "+format, args...) }
	}

	var wg sync.WaitGroup
	var pongErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer c2.Close()
		pongErr = runPeer(c2, "pong", false, cfg.UpgradePong, cfg, tag("pong"))
	}()

	pingErr := runPeer(c1, "ping", true, cfg.UpgradePing, cfg, tag("ping"))
	c1.Close()
	wg.Wait()

	if pingErr != nil {
		return pingErr
	}
	return pongErr
}
