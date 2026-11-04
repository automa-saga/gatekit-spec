package main

// Record is one stored key/value. Format is "v1" or "v2".
type Record struct {
	Key, Value, Format string
}

// Store stands in for a node's on-disk records plus its persisted active activation
// version. It implements migration.Store (Current / SetActive), so the same value
// drives both feature resolution and the migration.
type Store struct {
	Records []Record
	active  int
}

// NewStore returns a store seeded with two v1 records, at activation version 5.
func NewStore() *Store {
	return &Store{
		active:  5,
		Records: []Record{{"alpha", "1", "v1"}, {"beta", "2", "v1"}},
	}
}

// Current returns the currently-active activation version.
func (s *Store) Current() int { return s.active }

// SetActive persists a new active activation version (the finalize step).
func (s *Store) SetActive(version int) error { s.active = version; return nil }

// Formats returns each record's format, for logging.
func (s *Store) Formats() []string {
	out := make([]string, len(s.Records))
	for i, r := range s.Records {
		out[i] = r.Format
	}
	return out
}
