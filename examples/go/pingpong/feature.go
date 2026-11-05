package main

import "github.com/automa-saga/gatekit"

// MessageFormatV2 gates the new message wire format. It is declared once as a typed
// constant and used at both registration and the call site. The identifier is
// CamelCase, so it maps cleanly onto an env-var override name (MESSAGE_FORMAT_V2).
const MessageFormatV2 gatekit.Feature = "MessageFormatV2"

// A peer's binary version is modeled by which specs it knows:
//
//   - specsOld is the OLD binary. It knows MessageFormatV2 only as dormant at activation
//     5, so its MaxActivation is 5: it can neither send nor read v2.
//   - specsNew is the NEW binary. It adds the v6 GA spec, so its MaxActivation is 6: it
//     can read v2, and sends v2 once its active version reaches 6.
//
// Upgrading a peer swaps specsOld for specsNew (see peer.maybeUpgrade); the peer keeps
// the same activation version across the swap.
var (
	specsOld = gatekit.VersionedSpecs{
		{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
	}
	specsNew = gatekit.VersionedSpecs{
		{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
		{ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
	}
)

// newGate builds a peer's gate for the given binary version, resolving against src.
func newGate(upgraded bool, src gatekit.VersionSource) (*gatekit.FeatureGate, error) {
	specs := specsOld
	if upgraded {
		specs = specsNew
	}
	return gatekit.New(gatekit.Registry{
		MessageFormatV2: {
			Description: "Exchange messages in the v2 wire format (adds a checksum)",
			Specs:       specs,
		},
	}, src)
}

// activation is a peer's own activation version — the counter it finalizes. Each peer
// owns its copy; they coordinate the flip to 6 over the wire (see peer.go). It
// implements gatekit.VersionSource, so a peer's gate resolves against its live value:
// mutate active and the gate's answer changes, with no new gate.
type activation struct{ active int }

// Current implements gatekit.VersionSource.
func (a *activation) Current() gatekit.Activation { return gatekit.Activation(a.active) }
