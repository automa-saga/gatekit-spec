package main

import "github.com/automa-saga/gatekit"

// KV is the key-value store's API surface: Put and Get over a Store, with the write
// format chosen by the gate. It is what a request handler would call.
type KV struct {
	store *Store
	gate  *gatekit.FeatureGate
}

// NewKV builds a KV over store, with a gate that resolves RecordFormatV2 against the
// store's live active version. Because the gate reads the store, a finalize that
// advances active flips Put from writing v1 to writing v2 — with no new gate and no
// redeploy.
func NewKV(store *Store) (*KV, error) {
	gate, err := gatekit.New(Registry, version{store})
	if err != nil {
		return nil, err
	}
	return &KV{store: store, gate: gate}, nil
}

// Put writes key=value, encoding it in the format the gate currently selects: v1 while
// RecordFormatV2 is dormant, v2 once it is active. The call site never names a version
// — it asks the gate.
func (kv *KV) Put(key, value string) Record {
	var stored string
	if kv.gate.Enabled(RecordFormatV2) {
		stored = encodeV2(value)
	} else {
		stored = encodeV1(value)
	}
	kv.store.put(key, stored)
	return Record{Key: key, Stored: stored}
}

// Get reads key back, decoding whichever format it was stored in and validating a v2
// checksum. Reading does not care which format wrote the record — that is what keeps
// v1 and v2 records usable side by side.
func (kv *KV) Get(key string) (value string, ok bool, err error) {
	stored, ok := kv.store.get(key)
	if !ok {
		return "", false, nil
	}
	value, err = decode(stored)
	return value, true, err
}
