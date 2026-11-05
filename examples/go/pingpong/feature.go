package main

import "github.com/automa-saga/gatekit"

// MessageFormatV2 gates the new message wire format. It is declared once as a typed
// constant and used at both registration and the call site. The identifier is
// CamelCase, so it maps cleanly onto an env-var override name (MESSAGE_FORMAT_V2).
const MessageFormatV2 gatekit.Feature = "MessageFormatV2"

// registry is the peers' feature table: MessageFormatV2 is born dormant (Beta, off) at
// activation 5 and turns on (GA, on) at 6.
var registry = gatekit.Registry{
	MessageFormatV2: {
		Description: "Exchange messages in the v2 wire format (adds a checksum)",
		Specs: gatekit.VersionedSpecs{
			{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
			{ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
		},
	},
}

// activation is a peer's own activation version — the counter it finalizes. Each peer
// owns its copy; they coordinate the flip to 6 over the wire (see peer.go). It
// implements gatekit.VersionSource, so a peer's gate resolves against its live value:
// mutate active and the gate's answer changes, with no new gate.
type activation struct{ active int }

// Current implements gatekit.VersionSource.
func (a *activation) Current() gatekit.Activation { return gatekit.Activation(a.active) }
