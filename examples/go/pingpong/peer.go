package main

import (
	"fmt"

	"github.com/automa-saga/gatekit"
)

// peer is one participant. canV2 is whether it runs the NEW binary (and so is capable
// of v2 at all); gate decides whether, at the current shared cluster version, it
// should actually use v2.
type peer struct {
	name  string
	canV2 bool
	gate  *gatekit.FeatureGate
}

// newPeer builds a peer whose gate resolves against the shared cluster version.
func newPeer(name string, canV2 bool, c *cluster) (*peer, error) {
	gate, err := gatekit.New(registry, clusterVersion{c})
	if err != nil {
		return nil, err
	}
	return &peer{name: name, canV2: canV2, gate: gate}, nil
}

// speaksV2 is true only when this peer both can (new binary) and should (the gate
// resolves the feature on at the current cluster version) use v2. An upgraded peer at
// an un-finalized cluster version returns false — capable but dormant.
func (p *peer) speaksV2() bool {
	return p.canV2 && p.gate.Enabled(MessageFormatV2)
}

// encode renders a message in the format this peer currently speaks.
func (p *peer) encode(m Msg) string {
	if p.speaksV2() {
		return encodeV2(m)
	}
	return encodeV1(m)
}

// decode accepts a message. A new binary understands both v1 and v2 (that is what
// makes a staggered rollout safe); an old binary can only read v1 and errors on v2 —
// which is exactly why an upgraded peer must not send v2 before the coordinated flip.
func (p *peer) decode(wire string) (version string, err error) {
	switch wireVersion(wire) {
	case "v2":
		if !p.canV2 {
			return "", fmt.Errorf("%s runs the OLD binary and cannot read a v2 message: %q", p.name, wire)
		}
		return "v2", nil
	case "v1":
		return "v1", nil
	default:
		return "", fmt.Errorf("%s received an unrecognized message: %q", p.name, wire)
	}
}
