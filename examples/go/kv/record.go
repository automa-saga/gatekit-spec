package main

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
)

// Record is one stored key/value pair. Stored is the record exactly as it sits on
// disk — a serialized string in either the v1 or the v2 format. The format is part of
// the bytes, so a reader can tell v1 from v2 without any out-of-band flag.
type Record struct {
	Key    string
	Stored string
}

// Format reports which on-disk format a record is in ("v1", "v2", or "" if the bytes
// are unrecognized), without decoding the value.
func (r Record) Format() string { return formatOf(r.Stored) }

// encodeV1 is the original on-disk format: a "v1" tag and the raw value.
//
//	v1|<value>
func encodeV1(value string) string {
	return "v1|" + value
}

// encodeV2 is the new on-disk format: the same value plus a CRC32 integrity checksum.
// A v2 reader verifies the checksum on read, so a truncated or corrupted record is
// caught instead of served. This is why the format change is worth making — and why an
// old binary, which cannot read v2, must not be handed v2 records until the upgrade is
// committed.
//
//	v2|<value>|<crc32-hex>
func encodeV2(value string) string {
	return fmt.Sprintf("v2|%s|%08x", value, crc32.ChecksumIEEE([]byte(value)))
}

// formatOf returns the format tag at the front of a stored record.
func formatOf(stored string) string {
	switch {
	case strings.HasPrefix(stored, "v2|"):
		return "v2"
	case strings.HasPrefix(stored, "v1|"):
		return "v1"
	default:
		return ""
	}
}

// decode reads a stored record back into its value, validating a v2 checksum. It is
// the counterpart to encodeV1/encodeV2: whichever format wrote the record, decode
// reads it — which is what lets records in both formats coexist during a rollout.
func decode(stored string) (value string, err error) {
	switch formatOf(stored) {
	case "v1":
		return strings.TrimPrefix(stored, "v1|"), nil
	case "v2":
		parts := strings.SplitN(strings.TrimPrefix(stored, "v2|"), "|", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("malformed v2 record: %q", stored)
		}
		value = parts[0]
		want, err := strconv.ParseUint(parts[1], 16, 32)
		if err != nil {
			return "", fmt.Errorf("malformed v2 checksum in %q: %w", stored, err)
		}
		if got := crc32.ChecksumIEEE([]byte(value)); uint32(want) != got {
			return "", fmt.Errorf("v2 checksum mismatch for %q: stored %08x, computed %08x", stored, want, got)
		}
		return value, nil
	default:
		return "", fmt.Errorf("unrecognized record format: %q", stored)
	}
}
