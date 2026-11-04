package main

import "github.com/automa-saga/gatekit"

// MessageFormatV2 gates the new message wire format. Born dormant (off) at activation
// 5, on at 6.
const MessageFormatV2 gatekit.Feature = "message-format-v2"

// registry is the peers' shared feature table.
var registry = gatekit.Registry{
	MessageFormatV2: {
		Description: "Exchange messages in the v2 wire format (adds a checksum)",
		Specs: gatekit.VersionedSpecs{
			{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
			{ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
		},
	},
}

// cluster holds the one activation version the whole fleet shares — the "cluster
// version." Every peer resolves its gate against this, so a single finalize flips the
// fleet together (no peer can diverge on its own).
type cluster struct{ active int }

// clusterVersion adapts the shared cluster version into a gatekit.VersionSource.
type clusterVersion struct{ c *cluster }

// Current implements gatekit.VersionSource.
func (v clusterVersion) Current() gatekit.Activation { return gatekit.Activation(v.c.active) }
