package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/automa-saga/gatekit"
)

// Config tunes a run. It models a rolling deploy followed by a negotiated flip:
//   - UpgradePing / UpgradePong: the message number at which each peer is upgraded to
//     the new binary (so it can read v2). They differ, so the deploy is staggered.
//   - Lead: once both peers advertise Max >= 6, the first to notice proposes a flip that
//     many messages ahead. The number is carried on the wire, so both agree on it in-band.
//   - Total: stop after this many messages. Delay: pause before each reply.
type Config struct {
	UpgradePing int
	UpgradePong int
	Lead        int
	Total       int
	Delay       time.Duration
}

// peer is one process's state across the rollout. The gate's registry models the binary
// version: the old binary's registry stops at activation 5 (Max=5, cannot read or send
// v2); upgrading swaps in the new registry (Max=6). act is the activation version the
// gate resolves against — it governs only what the peer SENDS: v1 while active is 5, v2
// once it is 6.
//
// Capability and send-format are independent on purpose. Max >= 6 means you CAN read v2;
// advancing active to 6 means you now SEND v2. A peer spends the middle of the rollout
// upgraded (Max=6) but still sending v1 (active=5) — the mixed-version window that keeps a
// staggered deploy safe.
type peer struct {
	name      string
	upgradeAt int
	act       *activation
	gate      *gatekit.FeatureGate
}

// newPeer builds a peer running the OLD binary (Max=5) and dormant at activation 5, so it
// starts by sending v1 and cannot read v2.
func newPeer(name string, upgradeAt int) (*peer, error) {
	act := &activation{active: 5}
	gate, err := newGate(false, act)
	if err != nil {
		return nil, err
	}
	return &peer{name: name, upgradeAt: upgradeAt, act: act, gate: gate}, nil
}

// ev is the peer's advertised activation triple.
func (p *peer) ev() gatekit.EffectiveVersion { return p.gate.EffectiveVersion() }

// canReadV2 reports whether this peer runs the new binary — Max >= 6 in its triple.
func (p *peer) canReadV2() bool { return int(p.ev().Max) >= 6 }

// speaksV2 reports whether this peer should SEND v2 now — whether the gate resolves
// MessageFormatV2 on at the peer's current activation version.
func (p *peer) speaksV2() bool { return p.gate.Enabled(MessageFormatV2) }

// encode renders a message in the format this peer currently speaks.
func (p *peer) encode(m Msg) string {
	if p.speaksV2() {
		return encodeV2(m)
	}
	return encodeV1(m)
}

// maybeUpgrade performs the rolling deploy: once the conversation reaches this peer's
// scheduled point, its binary is swapped for the new one (Max rises to 6) and it can read
// v2. It still keeps sending v1 until the negotiated flip.
func (p *peer) maybeUpgrade(seq int, logf func(string, ...any)) error {
	if p.canReadV2() || seq < p.upgradeAt {
		return nil
	}
	gate, err := newGate(true, p.act)
	if err != nil {
		return err
	}
	p.gate = gate
	ev := p.ev()
	logf("-- upgraded to the new binary at message %d: supports %d/%d/%d (reads v1 AND v2; still sending v1)",
		seq, ev.Min, ev.Max, ev.Active)
	return nil
}

// runPeer plays the volley over conn until Total messages have passed. The initiator
// sends the opening message; both then reply to whatever they receive. It returns an
// error only if a message ever fails to decode — which, if the rollout is correct, never
// happens.
func runPeer(conn net.Conn, name string, initiator bool, upgradeAt int, cfg Config, logf func(string, ...any)) error {
	p, err := newPeer(name, upgradeAt)
	if err != nil {
		return err
	}
	r := bufio.NewReader(conn)
	partnerMax := 0 // the newest activation the partner supports; >= 6 means it can read v2
	agreed := 0     // the negotiated flip message number; 0 until proposed

	// send emits one message, performing the rolling upgrade, the flip proposal, and the
	// flip itself at the right moments.
	send := func(seq int) error {
		if err := p.maybeUpgrade(seq, logf); err != nil {
			return err
		}

		// Once both peers run the new binary (Max >= 6) and nobody has proposed yet,
		// propose a flip a few messages ahead and carry the number on the wire.
		if p.canReadV2() && partnerMax >= 6 && agreed == 0 {
			agreed = seq + cfg.Lead
			logf("-- both peers now support v2 (Max=6): proposing flip at message %d", agreed)
		}
		// At the agreed message, advance active to 6 so the gate turns v2 on for sending.
		if agreed > 0 && seq >= agreed && p.act.active < 6 {
			p.act.active = 6
			logf("-- reached the agreed message %d: flipping to v2 (sending v2 now)", agreed)
		}

		ev := p.ev()
		wire := p.encode(Msg{From: name, Seq: seq, Min: int(ev.Min), Max: int(ev.Max), Active: int(ev.Active), Flip: agreed})
		logf(">> seq=%2d  %s", seq, wire)
		_, werr := fmt.Fprintln(conn, wire)
		return werr
	}

	if initiator {
		if err := send(1); err != nil {
			return err
		}
	}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil // partner finished and closed the connection
			}
			return err
		}
		wire := line[:len(line)-1]
		m, format, derr := decode(wire)
		if derr != nil {
			return fmt.Errorf("%s could not read a message: %w", name, derr)
		}
		// The old binary cannot read v2. The rollout guarantees nobody is sent v2 before
		// supporting it; this check makes that invariant explicit.
		if format == "v2" && !p.canReadV2() {
			return fmt.Errorf("%s runs the OLD binary (Max=%d) and cannot read a v2 message: %q", name, p.ev().Max, wire)
		}
		partnerMax = m.Max
		// Adopt the partner's proposed flip (the later number wins, which is always safe
		// because both keep sending v1 until the flip).
		if m.Flip > agreed {
			agreed = m.Flip
			logf("-- agreed to flip to v2 at message %d", agreed)
		}

		note := ""
		if format == "v2" && !p.speaksV2() {
			note = "  (we read v2 though we still send v1 — decode both, send one)"
		}
		logf("<< seq=%2d  %s%s", m.Seq, wire, note)

		if m.Seq >= cfg.Total {
			logf("-- done after %d messages; no message ever failed to decode", m.Seq)
			return nil
		}
		if cfg.Delay > 0 {
			time.Sleep(cfg.Delay)
		}
		if err := send(m.Seq + 1); err != nil {
			return err
		}
	}
}
