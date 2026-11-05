package main

import (
	"context"

	"github.com/automa-saga/gatekit/migration"
)

// RewriteToV2 is the data migration tied to activation version 6: it rewrites every
// record from v1 to v2. The format change is also a wire change, so its scope is
// shared — it is run-at-finalize for a coordinated finalize, not run at install.
type RewriteToV2 struct{ Store *Store }

// ID uniquely identifies the migration (stable across releases).
func (RewriteToV2) ID() string { return "rewrite-records-v2" }

// Description is a short human-readable summary.
func (RewriteToV2) Description() string { return "Rewrite all records from v1 to v2" }

// Version is the activation version this migration activates with.
func (RewriteToV2) Version() int { return 6 }

// Scope marks this shared, so the planner holds it for finalize.
func (RewriteToV2) Scope() migration.Scope { return migration.Shared }

// Applies reads ground truth (any record still v1), not a ledger, so a re-run after a
// crash is safe and becomes a no-op once every record is already v2.
func (m RewriteToV2) Applies(context.Context) (bool, error) {
	for _, r := range m.Store.Records {
		if r.Format == "v1" {
			return true, nil
		}
	}
	return false, nil
}

// Execute performs the forward transformation.
func (m RewriteToV2) Execute(context.Context) error {
	for i := range m.Store.Records {
		m.Store.Records[i].Format = "v2"
	}
	return nil
}

// Rollback reverses Execute (used to compensate on failure, and for a reversible
// downgrade).
func (m RewriteToV2) Rollback(context.Context) error {
	for i := range m.Store.Records {
		m.Store.Records[i].Format = "v1"
	}
	return nil
}
