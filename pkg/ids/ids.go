package ids

import (
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

// ID represents a unique identifier, standardized across iotp as UUIDv7.
type ID = uuid.UUID

// Nil represents the zero/empty UUID.
var Nil = uuid.Nil

// New generates a new time-ordered UUIDv7.
func New() (ID, error) {
	return uuid.NewV7()
}

// MustNew generates a new UUIDv7, panicking only if entropy generation fails.
func MustNew() ID {
	id, err := uuid.NewV7()
	if err != nil {
		panic("ids: failed to generate UUIDv7: " + err.Error())
	}
	return id
}

// Parse parses a UUID string into an ID.
func Parse(s string) (ID, error) {
	return uuid.Parse(s)
}

// MustParse parses a UUID string or panics if invalid.
func MustParse(s string) ID {
	return uuid.MustParse(s)
}

// IsValid checks whether a string is a valid UUID representation.
func IsValid(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// TimestampMs extracts the 48-bit Unix epoch millisecond timestamp embedded in a UUIDv7.
func TimestampMs(id ID) int64 {
	// UUIDv7 embeds 48-bit timestamp in big-endian bytes 0..5
	b := id[:]
	ts := int64(binary.BigEndian.Uint32(b[0:4]))<<16 | int64(binary.BigEndian.Uint16(b[4:6]))
	return ts
}

// Time converts the embedded timestamp in a UUIDv7 to a time.Time in UTC.
func Time(id ID) time.Time {
	return time.UnixMilli(TimestampMs(id)).UTC()
}
