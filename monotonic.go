package ulid

import (
	"bufio"
	"io"
	"math"
	"sync"
)

// MonotonicReader is an interface for entropy sources that provide monotonically
// increasing random bytes within the same millisecond. The standard [io.Reader]
// interface is used when a new millisecond is detected. [MonotonicRead] is
// called when the same millisecond is detected, allowing implementations to
// increment the previous entropy value.
type MonotonicReader interface {
	io.Reader
	MonotonicRead(ms uint64, p []byte) error
}

// MonotonicEntropy is an entropy source that provides monotonically increasing
// entropy values within the same Unix millisecond. It wraps an underlying
// [io.Reader] as the source of randomness.
//
// When a new millisecond is detected, fresh entropy is read from the underlying
// source. When the same millisecond is detected, the previous entropy value is
// incremented by a random amount between 1 and inc (inclusive).
//
// MonotonicEntropy is NOT safe for concurrent use. Wrap it with
// [LockedMonotonicReader] for thread safety.
type MonotonicEntropy struct {
	r       io.Reader
	ms      uint64
	inc     uint64
	entropy uint80
	rand    [8]byte
	rng     rng
}

// rng is an optional interface for fast random number generation. When the
// underlying entropy source implements this (e.g., *math/rand.Rand), the
// increment is computed directly rather than reading random bytes.
type rng interface {
	Int63n(n int64) int64
}

// Monotonic creates a new [MonotonicEntropy] source wrapping the given entropy
// reader. The inc parameter controls the maximum random increment per ULID
// generated within the same millisecond. If inc is 0, it defaults to
// [math.MaxUint32], which provides a good balance between unpredictability
// and entropy space utilization.
//
// The entropy reader is wrapped in a [bufio.Reader] to minimize read syscalls.
func Monotonic(entropy io.Reader, inc uint64) *MonotonicEntropy {
	m := MonotonicEntropy{
		r:   bufio.NewReader(entropy),
		inc: inc,
	}
	if inc == 0 {
		m.inc = math.MaxUint32
	}
	m.rng, _ = entropy.(rng)
	return &m
}

// MonotonicRead generates monotonically increasing entropy for the given
// millisecond. If ms equals the previous call's millisecond, the entropy is
// incremented. Otherwise, fresh random entropy is read from the underlying
// source. Returns [ErrMonotonicOverflow] if the entropy space is exhausted.
func (m *MonotonicEntropy) MonotonicRead(ms uint64, p []byte) error {
	if !m.entropy.isZero() && m.ms == ms {
		inc, err := m.increment()
		if err != nil {
			return err
		}
		if overflow := m.entropy.add(inc); overflow {
			return ErrMonotonicOverflow
		}
		m.entropy.writeTo(p)
		return nil
	}

	if _, err := io.ReadFull(m.r, p); err != nil {
		return err
	}

	m.ms = ms
	m.entropy.readFrom(p)
	return nil
}

// Read implements [io.Reader]. It reads random bytes from the underlying source.
func (m *MonotonicEntropy) Read(p []byte) (int, error) {
	return m.r.Read(p)
}

// increment returns a random value between 1 and m.inc (inclusive).
func (m *MonotonicEntropy) increment() (uint64, error) {
	if m.rng != nil {
		return 1 + uint64(m.rng.Int63n(int64(m.inc))), nil
	}
	if _, err := io.ReadFull(m.r, m.rand[:]); err != nil {
		return 0, err
	}
	// Mask to get a value in [0, m.inc) then add 1.
	return 1 + (uint64(m.rand[0])<<56|
		uint64(m.rand[1])<<48|
		uint64(m.rand[2])<<40|
		uint64(m.rand[3])<<32|
		uint64(m.rand[4])<<24|
		uint64(m.rand[5])<<16|
		uint64(m.rand[6])<<8|
		uint64(m.rand[7]))%(m.inc), nil
}

// LockedMonotonicReader wraps a [MonotonicReader] with a mutex for safe
// concurrent use.
type LockedMonotonicReader struct {
	mu sync.Mutex
	MonotonicReader
}

// MonotonicRead implements [MonotonicReader] with mutex synchronization.
func (lr *LockedMonotonicReader) MonotonicRead(ms uint64, p []byte) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	return lr.MonotonicReader.MonotonicRead(ms, p)
}

// Read implements [io.Reader] with mutex synchronization.
func (lr *LockedMonotonicReader) Read(p []byte) (int, error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	return lr.MonotonicReader.Read(p)
}

// uint80 represents an unsigned 80-bit integer used to track the monotonic
// entropy counter. It is stored as a high 16-bit word and a low 64-bit word,
// matching the layout of the entropy portion of a ULID.
type uint80 struct {
	hi uint16
	lo uint64
}

// add adds n to the 80-bit value and returns true if overflow occurred.
func (u *uint80) add(n uint64) (overflow bool) {
	lo := u.lo
	u.lo += n
	if u.lo < lo {
		hi := u.hi
		u.hi++
		return u.hi < hi
	}
	return false
}

// isZero returns true if the value is zero.
func (u uint80) isZero() bool {
	return u.hi == 0 && u.lo == 0
}

// readFrom reads a 10-byte big-endian representation into the uint80.
func (u *uint80) readFrom(p []byte) {
	u.hi = uint16(p[0])<<8 | uint16(p[1])
	u.lo = uint64(p[2])<<56 |
		uint64(p[3])<<48 |
		uint64(p[4])<<40 |
		uint64(p[5])<<32 |
		uint64(p[6])<<24 |
		uint64(p[7])<<16 |
		uint64(p[8])<<8 |
		uint64(p[9])
}

// writeTo writes the uint80 as a 10-byte big-endian representation.
func (u uint80) writeTo(p []byte) {
	p[0] = byte(u.hi >> 8)
	p[1] = byte(u.hi)
	p[2] = byte(u.lo >> 56)
	p[3] = byte(u.lo >> 48)
	p[4] = byte(u.lo >> 40)
	p[5] = byte(u.lo >> 32)
	p[6] = byte(u.lo >> 24)
	p[7] = byte(u.lo >> 16)
	p[8] = byte(u.lo >> 8)
	p[9] = byte(u.lo)
}

