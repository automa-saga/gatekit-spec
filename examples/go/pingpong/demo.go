package main

import "fmt"

// Run plays out the rolling upgrade: ping and pong take turns sending a message, while
// a coordinator rolls out the new binary peer by peer and then finalizes. Each stage
// is reported via logf. It returns an error only if a peer ever fails to decode a
// message it was sent — which, if the model is right, never happens.
func Run(logf func(string, ...any)) error {
	c := &cluster{active: 5}
	ping, err := newPeer("ping", false, c) // both start on the OLD binary
	if err != nil {
		return err
	}
	pong, err := newPeer("pong", false, c)
	if err != nil {
		return err
	}

	logf("start   both on the OLD binary; cluster active=5 → both speak v1")

	peers := [2]*peer{ping, pong}
	const rounds = 10
	for round := 1; round <= rounds; round++ {
		// The coordinator applies fleet changes before this round.
		switch round {
		case 3:
			pong.canV2 = true
			logf("rollout pong upgraded to the NEW binary (v2-capable); active still 5, so it stays dormant and keeps sending v1")
		case 5:
			ping.canV2 = true
			logf("rollout ping upgraded too → every peer supports v2 (readiness); still active=5, still sending v1")
		case 7:
			c.active = 6
			logf("FINALIZE cluster active 5 → 6; message-format-v2 is ON for the whole fleet")
		}

		sender := peers[(round-1)%2]
		receiver := peers[round%2]

		m := Msg{From: sender.name, Seq: round, Key: "k", Value: fmt.Sprintf("%d", round)}
		wire := sender.encode(m)
		ver, err := receiver.decode(wire)
		if err != nil {
			return err // a decode failure is a broken rollout
		}

		note := ""
		if sender.canV2 && !sender.speaksV2() {
			note = "  (sender is v2-capable but still dormant → v1)"
		}
		logf("round %2d %-4s → %-4s  %-34q  %s reads %s%s",
			round, sender.name, receiver.name, wire, receiver.name, ver, note)
	}

	logf("done    no peer ever failed to read a message — the rollout was safe")
	return nil
}
