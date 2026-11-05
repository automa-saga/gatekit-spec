package main

// Store is the node's persisted state: its records and its active activation version.
// It implements migration.Store (Current / SetActive), so the one value that finalize
// advances is the same value the gate resolves against.
//
// It is in-memory to keep the example runnable anywhere; a real kv would back this with
// a file or an embedded database. The manual's "Implementing activation" chapter shows
// a crash-safe file-backed migration.Store with the same two methods.
type Store struct {
	records []Record
	active  int
}

// NewStore returns an empty store at activation version 5 — the new binary is deployed,
// but RecordFormatV2 is still dormant, so writes land in v1.
func NewStore() *Store {
	return &Store{active: 5}
}

// Current implements migration.Store: the currently-active activation version.
func (s *Store) Current() int { return s.active }

// SetActive implements migration.Store: persist a new active version (the finalize
// step). A real implementation would write this atomically and refuse to move backward.
func (s *Store) SetActive(version int) error { s.active = version; return nil }

// put writes (or overwrites) a record's stored bytes.
func (s *Store) put(key, stored string) {
	for i := range s.records {
		if s.records[i].Key == key {
			s.records[i].Stored = stored
			return
		}
	}
	s.records = append(s.records, Record{Key: key, Stored: stored})
}

// get returns a record's stored bytes.
func (s *Store) get(key string) (stored string, ok bool) {
	for _, r := range s.records {
		if r.Key == key {
			return r.Stored, true
		}
	}
	return "", false
}

// Records returns the stored records, in insertion order.
func (s *Store) Records() []Record { return s.records }

// Formats returns each record's on-disk format, for logging.
func (s *Store) Formats() []string {
	out := make([]string, len(s.records))
	for i, r := range s.records {
		out[i] = r.Format()
	}
	return out
}
