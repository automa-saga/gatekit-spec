package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// parseSends pulls (seq, format) from the ">> seq=" send lines in a captured log.
func parseSends(t *testing.T, log []string) []struct {
	seq    int
	format string
} {
	t.Helper()
	var sends []struct {
		seq    int
		format string
	}
	for _, l := range log {
		i := strings.Index(l, ">> seq=")
		if i < 0 {
			continue
		}
		var seq int
		if _, err := fmt.Sscanf(l[i:], ">> seq=%d", &seq); err != nil {
			t.Fatalf("could not parse seq from %q", l)
		}
		format := ""
		switch {
		case strings.Contains(l, " v2|"):
			format = "v2"
		case strings.Contains(l, " v1|"):
			format = "v1"
		}
		sends = append(sends, struct {
			seq    int
			format string
		}{seq, format})
	}
	return sends
}

// runCaptured drives the demo and returns the captured log lines.
func runCaptured(t *testing.T, cfg Config) ([]string, error) {
	t.Helper()
	var mu sync.Mutex
	var log []string
	err := runDemo(cfg, func(format string, args ...any) {
		mu.Lock()
		log = append(log, fmt.Sprintf(format, args...))
		mu.Unlock()
	})
	return log, err
}

// TestDemo drives the in-process demo and checks the rolling-upgrade invariants:
// nobody ever fails to decode a message, the format flips exactly once (one-way), it
// flips only after both peers are upgraded, and the run starts on v1 and ends on v2.
func TestDemo(t *testing.T) {
	cfg := Config{UpgradePing: 5, UpgradePong: 3, Lead: 2, Total: 12, Delay: 0}
	log, err := runCaptured(t, cfg)
	if err != nil {
		t.Fatalf("demo failed (a message did not decode): %v", err)
	}
	sends := parseSends(t, log)
	if len(sends) == 0 {
		t.Fatal("no send lines captured")
	}

	// One-way flip: once a v2 send appears, no later send may be v1.
	firstV2 := -1
	for _, s := range sends {
		if s.format == "v2" && firstV2 < 0 {
			firstV2 = s.seq
		}
		if firstV2 >= 0 && s.format == "v1" && s.seq > firstV2 {
			t.Errorf("v1 send at seq=%d after the flip at seq=%d: the format must flip one way", s.seq, firstV2)
		}
	}
	if firstV2 < 0 {
		t.Fatal("the format never flipped to v2")
	}
	// The flip must not happen before the later peer (ping, at message 5) is upgraded.
	if firstV2 <= cfg.UpgradePing {
		t.Errorf("flipped to v2 at seq=%d, before ping was upgraded (message %d)", firstV2, cfg.UpgradePing)
	}
	if sends[0].format != "v1" {
		t.Errorf("first send = %s, want v1 (old binary)", sends[0].format)
	}
	if last := sends[len(sends)-1]; last.format != "v2" {
		t.Errorf("last send = %s, want v2", last.format)
	}

	joined := strings.Join(log, "\n")
	for _, want := range []string{
		"supports 5/6/5",                    // upgraded: Max rose to 6, still active=5
		"both peers now support v2 (Max=6)", // the Max>=6 handshake
		"proposing flip at message",
		"flipping to v2",
		"decode both, send one",
		fmt.Sprintf("done after %d messages", cfg.Total),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected the log to contain %q:\n%s", want, joined)
		}
	}
}

// TestHoldsWhenPeerNeverUpgrades confirms the safety guard: if one peer stays on the old
// binary (never upgraded within the run), no peer ever proposes or sends v2 — everyone
// keeps speaking v1, and nothing fails to decode.
func TestHoldsWhenPeerNeverUpgrades(t *testing.T) {
	cfg := Config{UpgradePing: 5, UpgradePong: 999, Lead: 2, Total: 12, Delay: 0}
	log, err := runCaptured(t, cfg)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	for _, s := range parseSends(t, log) {
		if s.format == "v2" {
			t.Errorf("v2 send at seq=%d, but pong never upgraded — must stay on v1", s.seq)
		}
	}
	if strings.Contains(strings.Join(log, "\n"), "flipping to v2") {
		t.Error("flipped to v2 even though a peer never upgraded")
	}
}

// TestUpgradeAndFlip is the unit-level version of the rollout. A fresh peer is the old
// binary: Max=5, cannot read v2, sends v1. Upgrading raises Max to 6 (can read v2) but it
// still sends v1 until active advances. Advancing active to 6 flips its send format to v2.
func TestUpgradeAndFlip(t *testing.T) {
	p, err := newPeer("ping", 3)
	if err != nil {
		t.Fatal(err)
	}
	if int(p.ev().Max) != 5 || p.canReadV2() {
		t.Fatalf("old binary should have Max=5 and not read v2, got Max=%d", p.ev().Max)
	}
	if p.speaksV2() {
		t.Fatal("old binary must not speak v2")
	}

	if err := p.maybeUpgrade(3, func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	if int(p.ev().Max) != 6 || !p.canReadV2() {
		t.Fatalf("upgraded binary should have Max=6 and read v2, got Max=%d", p.ev().Max)
	}
	if p.speaksV2() {
		t.Fatal("upgraded-but-dormant peer (active=5) must still send v1")
	}

	p.act.active = 6 // what the agreed flip does
	if !p.speaksV2() {
		t.Fatal("at active=6 an upgraded peer must speak v2")
	}
	if _, format, _ := decode(p.encode(Msg{From: "ping", Seq: 9})); format != "v2" {
		t.Fatalf("at active=6 encode = %s, want v2", format)
	}
}

// TestDecodeDetectsCorruption confirms the v2 checksum is actually checked: a tampered
// message fails to decode instead of being acted on.
func TestDecodeDetectsCorruption(t *testing.T) {
	wire := encodeV2(Msg{From: "pong", Seq: 8, Min: 5, Max: 6, Active: 6, Flip: 7})
	tampered := strings.Replace(wire, "seq=8", "seq=9", 1) // body changes, checksum does not
	if _, _, err := decode(tampered); err == nil {
		t.Fatal("decode should have reported a checksum mismatch, got nil error")
	}
}

// TestDecodeRejectsMissingField confirms a v1 message (which has no checksum) is rejected
// when a required field is missing, rather than read with a zero-valued field.
func TestDecodeRejectsMissingField(t *testing.T) {
	if _, _, err := decode("v1|from=ping|ev=5/5/5|flip=0"); err == nil { // no seq
		t.Fatal("decode should reject a message missing seq, got nil error")
	}
	// A complete message still decodes.
	if _, _, err := decode("v1|from=ping|seq=1|ev=5/5/5|flip=0"); err != nil {
		t.Fatalf("decode of a complete v1 message failed: %v", err)
	}
}
