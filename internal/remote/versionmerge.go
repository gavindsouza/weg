/*
Copyright © 2025 Gavin <me@gavv.in>

k-way merge of per-doctype Version JSONL streams into global chronological order.
*/
package remote

import (
	"encoding/json"
	"io"
)

// mergeVersions reads VersionRecords from each reader (one JSON value per line,
// each stream already sorted by creation asc) and calls emit for every record in
// global order by (Creation, Name). Memory is bounded to one buffered record per
// stream regardless of total version count.
//
// ponytail: linear min-scan over streams — with ~10 doctypes a heap buys nothing.
// A stream that fails to decode (e.g. a truncated trailing line) is treated as
// exhausted; its error is returned after all other streams drain.
func mergeVersions(readers []io.Reader, emit func(VersionRecord) error) error {
	type stream struct {
		dec  *json.Decoder
		head *VersionRecord
		done bool
	}

	streams := make([]*stream, len(readers))
	for i, r := range readers {
		streams[i] = &stream{dec: json.NewDecoder(r)}
	}

	var firstErr error
	// advance loads the next record into s.head, or marks the stream done.
	advance := func(s *stream) {
		var v VersionRecord
		if err := s.dec.Decode(&v); err != nil {
			if err != io.EOF && firstErr == nil {
				firstErr = err
			}
			s.head, s.done = nil, true
			return
		}
		s.head = &v
	}

	for _, s := range streams {
		advance(s)
	}

	for {
		// Pick the stream with the smallest live head.
		var best *stream
		for _, s := range streams {
			if s.done {
				continue
			}
			if best == nil || lessVersion(*s.head, *best.head) {
				best = s
			}
		}
		if best == nil {
			return firstErr // all streams drained
		}
		if err := emit(*best.head); err != nil {
			return err
		}
		advance(best)
	}
}

// lessVersion orders by creation timestamp, then name for stability. Creation is
// a Frappe datetime string ("2024-01-02 03:04:05.123456") which sorts lexically.
func lessVersion(a, b VersionRecord) bool {
	if a.Creation != b.Creation {
		return a.Creation < b.Creation
	}
	return a.Name < b.Name
}
