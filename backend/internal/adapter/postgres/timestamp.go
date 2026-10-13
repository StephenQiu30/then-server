package postgres

import "time"

// persistedTimestamp matches pgx timestamptz encoding, so a write response and
// its database replay expose the same instant even on nanosecond clocks.
func persistedTimestamp(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}
