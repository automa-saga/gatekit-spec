package main

import (
	"fmt"
	"net"
	"time"
)

// serve is the pong side: it listens, accepts one ping, and plays the volley. pong does
// not initiate — it replies to whatever ping sends.
func serve(addr string, cfg Config, logf func(string, ...any)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	logf("pong listening on %s — now run `pingpong ping` in another terminal", addr)

	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	logf("-- ping connected")
	return runPeer(conn, "pong", false, cfg.UpgradePong, cfg, logf)
}

// dial is the ping side: it connects to a waiting pong and sends the opening message.
func dial(addr string, cfg Config, logf func(string, ...any)) error {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("could not connect to %s — start `pingpong pong` there first: %w", addr, err)
	}
	defer conn.Close()
	logf("-- connected to pong at %s", addr)
	return runPeer(conn, "ping", true, cfg.UpgradePing, cfg, logf)
}
