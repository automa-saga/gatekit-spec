package main

import (
	"fmt"
	"hash/crc32"
)

// Msg is what a peer sends: a kv record (Key/Value) plus who sent it and a sequence
// number. It is the same payload the kv capstone stores; here it travels on the wire.
type Msg struct {
	From  string
	Seq   int
	Key   string
	Value string
}

// encodeV1 is the old wire format: plain fields.
func encodeV1(m Msg) string {
	return fmt.Sprintf("v1 %s seq=%d %s=%s", m.From, m.Seq, m.Key, m.Value)
}

// encodeV2 is the new wire format: the same fields plus a checksum.
func encodeV2(m Msg) string {
	body := fmt.Sprintf("%s seq=%d %s=%s", m.From, m.Seq, m.Key, m.Value)
	return fmt.Sprintf("v2 %s crc=%08x", body, crc32.ChecksumIEEE([]byte(body)))
}

// wireVersion reports which format a wire message is in ("v1", "v2", or "" if
// unrecognized), without needing to fully parse it.
func wireVersion(wire string) string {
	switch {
	case len(wire) >= 3 && wire[:3] == "v2 ":
		return "v2"
	case len(wire) >= 3 && wire[:3] == "v1 ":
		return "v1"
	default:
		return ""
	}
}
