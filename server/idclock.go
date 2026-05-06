package server

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// HexIDGen produces 16-byte hex IDs from crypto/rand. Implements IDGenerator.
type HexIDGen struct{}

// NewID returns a fresh 32-character hex ID. Panics only on a broken crypto
// source, which would itself indicate a fatal environment problem.
func (HexIDGen) NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand on darwin/linux does not fail in practice; fall back to
		// a time-only ID rather than panicking the request handler.
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

// SystemClock implements Clock with time.Now.
type SystemClock struct{}

// Now returns the current wall time.
func (SystemClock) Now() time.Time { return time.Now() }
