package main

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
)

// Msg is one volley. From is the sender ("ping" or "pong"); Seq is the shared,
// incrementing conversation counter; Min/Max/Active is the sender's gatekit activation
// triple (gate.EffectiveVersion) — Max is the key signal, since Max >= 6 means the sender
// runs the new binary and can read v2; Flip is the negotiated message number at which to
// switch to v2 (0 = not agreed yet).
//
// The triple and Flip are the handshake: each peer advertises the versions it supports,
// and once both advertise Max >= 6, one proposes a Flip number on the wire. Both carry
// that number in their v1 messages until Seq reaches it — then they send v2. So no peer is
// ever handed a format it cannot read, and the switch point is agreed in-band rather than
// hard-coded.
type Msg struct {
	From   string
	Seq    int
	Min    int
	Max    int
	Active int
	Flip   int
}

// body renders the format-independent fields. Both v1 and v2 wrap the same body; v2
// adds a checksum over it. The triple travels as ev=min/max/active.
func (m Msg) body() string {
	return fmt.Sprintf("from=%s|seq=%d|ev=%d/%d/%d|flip=%d", m.From, m.Seq, m.Min, m.Max, m.Active, m.Flip)
}

// encodeV1 is the original wire format: a "v1" tag and the plain body.
//
//	v1|from=ping|seq=3|ev=5/5/5|flip=0
func encodeV1(m Msg) string {
	return "v1|" + m.body()
}

// encodeV2 is the new wire format: the same body plus a CRC32 checksum. A v2 reader
// verifies the checksum, so a corrupted or truncated message is caught rather than
// acted on — the reason the format change is worth making.
//
//	v2|from=ping|seq=8|ev=5/6/6|flip=7|crc=1a2b3c4d
func encodeV2(m Msg) string {
	body := m.body()
	return fmt.Sprintf("v2|%s|crc=%08x", body, crc32.ChecksumIEEE([]byte(body)))
}

// decode parses a wire message, validating a v2 checksum, and reports which format it
// was in. The new binary decodes both formats — that is what lets v1 and v2 messages
// coexist while the two peers roll over. (An *old* binary could not; runPeer models that
// by refusing a v2 message when the peer has not been upgraded yet.)
func decode(wire string) (m Msg, format string, err error) {
	parts := strings.Split(wire, "|")
	if len(parts) < 2 {
		return Msg{}, "", fmt.Errorf("unrecognized message: %q", wire)
	}
	format = parts[0]
	fields := parts[1:]

	switch format {
	case "v1":
		// fields are the body as-is.
	case "v2":
		last := fields[len(fields)-1]
		if !strings.HasPrefix(last, "crc=") {
			return Msg{}, "", fmt.Errorf("v2 message missing checksum: %q", wire)
		}
		body := strings.Join(fields[:len(fields)-1], "|")
		want, err := strconv.ParseUint(strings.TrimPrefix(last, "crc="), 16, 32)
		if err != nil {
			return Msg{}, "", fmt.Errorf("malformed v2 checksum in %q: %w", wire, err)
		}
		if got := crc32.ChecksumIEEE([]byte(body)); uint32(want) != got {
			return Msg{}, "", fmt.Errorf("v2 checksum mismatch for %q: stored %08x, computed %08x", wire, want, got)
		}
		fields = fields[:len(fields)-1]
	default:
		return Msg{}, "", fmt.Errorf("unrecognized message format %q in %q", format, wire)
	}

	for _, f := range fields {
		key, value, ok := strings.Cut(f, "=")
		if !ok {
			return Msg{}, "", fmt.Errorf("malformed field %q in %q", f, wire)
		}
		switch key {
		case "from":
			m.From = value
		case "seq":
			n, err := strconv.Atoi(value)
			if err != nil {
				return Msg{}, "", fmt.Errorf("malformed seq %q in %q", value, wire)
			}
			m.Seq = n
		case "ev":
			nums := strings.Split(value, "/")
			if len(nums) != 3 {
				return Msg{}, "", fmt.Errorf("malformed triple %q in %q", value, wire)
			}
			for i, dst := range []*int{&m.Min, &m.Max, &m.Active} {
				n, err := strconv.Atoi(nums[i])
				if err != nil {
					return Msg{}, "", fmt.Errorf("malformed triple %q in %q: %w", value, wire, err)
				}
				*dst = n
			}
		case "flip":
			n, err := strconv.Atoi(value)
			if err != nil {
				return Msg{}, "", fmt.Errorf("malformed flip %q in %q", value, wire)
			}
			m.Flip = n
		}
	}
	return m, format, nil
}
