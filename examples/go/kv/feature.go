package main

import "github.com/automa-saga/gatekit"

// RecordFormatV2 gates the new on-disk record format. It is declared once as a typed
// constant and used at both registration and the call site. The identifier is
// CamelCase, so it maps cleanly onto an env-var override name (RECORD_FORMAT_V2).
const RecordFormatV2 gatekit.Feature = "RecordFormatV2"

// Registry is kv's feature table: RecordFormatV2 is born dormant (Beta, off) at
// activation 5 and turns on (GA, on) at 6.
var Registry = gatekit.Registry{
	RecordFormatV2: {
		Description: "Store records in the v2 format (adds an integrity checksum)",
		Specs: gatekit.VersionedSpecs{
			{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
			{ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
		},
	},
}

// version adapts the Store's active version into a gatekit.VersionSource, so the gate
// resolves against the same value finalize advances.
type version struct{ store *Store }

// Current implements gatekit.VersionSource.
func (v version) Current() gatekit.Activation { return gatekit.Activation(v.store.active) }
