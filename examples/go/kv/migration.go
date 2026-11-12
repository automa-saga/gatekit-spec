package main

import (
	"context"

	"github.com/automa-saga/gatekit/migration"
)

// RewriteToV2 is the data migration tied to activation version 6: it rewrites every
// stored record from the v1 format to the v2 format. Its scope is Shared, so the
// planner holds it for a deliberate finalize rather than running it at install — once a
// record is v2, an older binary rolled back onto this data could not read it, so the
// rewrite waits until the upgrade is committed.
type RewriteToV2 struct{ Store *Store }

// ID uniquely identifies the migration (stable across releases).
func (RewriteToV2) ID() string { return "rewrite-records-v2" }

// Description is a short human-readable summary.
func (RewriteToV2) Description() string { return "Rewrite all records from v1 to v2" }

// Version is the activation version this migration activates with.
func (RewriteToV2) Version() int { return 6 }

// Scope marks this Shared, so the planner holds it for finalize.
func (RewriteToV2) Scope() migration.Scope { return migration.Shared }

// Applies reads ground truth (is any record still in v1?), not a ledger, so a re-run
// after a crash is safe and becomes a no-op once every record is already v2.
func (m RewriteToV2) Applies(context.Context) (bool, error) {
	for _, r := range m.Store.records {
		if r.Format() == "v1" {
			return true, nil
		}
	}
	return false, nil
}

// Execute performs the forward transformation: decode each v1 record and re-encode it
// in v2. It re-reads the value rather than string-patching the bytes, so the new
// checksum is computed from the real value.
func (m RewriteToV2) Execute(context.Context) error {
	for i := range m.Store.records {
		if m.Store.records[i].Format() != "v1" {
			continue
		}
		value, err := decode(m.Store.records[i].Stored)
		if err != nil {
			return err
		}
		m.Store.records[i].Stored = encodeV2(value)
	}
	return nil
}

// Rollback reverses Execute (used to compensate on failure, and for a reversible
// downgrade): decode each v2 record and re-encode it in v1.
func (m RewriteToV2) Rollback(context.Context) error {
	for i := range m.Store.records {
		if m.Store.records[i].Format() != "v2" {
			continue
		}
		value, err := decode(m.Store.records[i].Stored)
		if err != nil {
			return err
		}
		m.Store.records[i].Stored = encodeV1(value)
	}
	return nil
}

// Reversible declares that Rollback is a true inverse of Execute (v2 decodes back to the
// same value, re-encoded as v1), so lowering the active version across this migration is
// allowed (migration-spec §1). gatekit never infers this from the presence of Rollback —
// a stub would otherwise permit a data-losing retreat — so a reversible migration must say
// so explicitly.
func (RewriteToV2) Reversible() bool { return true }
