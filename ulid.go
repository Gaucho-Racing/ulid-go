// Package ulid implements the Universally Unique Lexicographically Sortable
// Identifier (ULID) specification as defined in https://github.com/ulid/spec.
//
// A ULID is a 128-bit identifier composed of a 48-bit Unix millisecond
// timestamp and 80 bits of cryptographic randomness, encoded as a 26-character
// lowercase Crockford Base32 string. ULIDs are designed to be a drop-in
// replacement for UUIDs with the added benefit of being lexicographically
// sortable by creation time.
//
// The simplest way to generate a ULID:
//
//	id := ulid.Make()
//	fmt.Println(id) // e.g., "01arz3ndektsv4rrffq69g5fav"
//
// For prefixed identifiers (useful for entity-scoped IDs):
//
//	id := ulid.Make()
//	fmt.Println(id.Prefixed("user")) // "user_01arz3ndektsv4rrffq69g5fav"
//
// For distributed systems, use a [Generator] with a node ID to guarantee
// uniqueness across nodes without coordination:
//
//	gen := ulid.NewGenerator(ulid.WithNodeID(42))
//	id := gen.Make()
package ulid

import (
	"crypto/rand"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"time"
)

// ULID is a 16-byte Universally Unique Lexicographically Sortable Identifier.
//
// The components are encoded as 16 octets in big-endian (network byte order):
//
//	Bytes [0:6]  - 48-bit Unix millisecond timestamp
//	Bytes [6:16] - 80-bit entropy (randomness or node-partitioned randomness)
type ULID [16]byte

const (
	// EncodedSize is the length of a ULID encoded as a string.
	EncodedSize = 26

	// BinarySize is the length of a ULID in binary form.
	BinarySize = 16

	// encoding is the lowercase Crockford Base32 alphabet used for ULID encoding.
	encoding = "0123456789abcdefghjkmnpqrstvwxyz"

	// maxTime is the maximum Unix timestamp in milliseconds that can be
	// encoded in a ULID's 48-bit timestamp field.
	maxTime = (1 << 48) - 1
)

// Zero is the zero-value ULID.
var Zero ULID

// Predefined errors returned by this package.
var (
	ErrDataSize          = errors.New("ulid: bad data size when unmarshaling")
	ErrInvalidCharacters = errors.New("ulid: bad data characters when unmarshaling")
	ErrBufferSize        = errors.New("ulid: bad buffer size when marshaling")
	ErrBigTime           = errors.New("ulid: timestamp too big")
	ErrOverflow          = errors.New("ulid: overflow when unmarshaling")
	ErrMonotonicOverflow = errors.New("ulid: monotonic entropy overflow")
	ErrScanValue         = errors.New("ulid: source value must be a string or byte slice")
	ErrInvalidPrefix     = errors.New("ulid: invalid prefix format")
)

// dec is a 256-byte lookup table that maps ASCII characters to their
// Crockford Base32 values. 0xFF indicates an invalid character.
var dec = [256]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x00
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x08
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x10
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x18
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x20
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x28
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // '0'-'7'
	0x08, 0x09, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // '8'-'9'
	0xFF, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, // 'A'-'G'
	0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14, 0x15, 0xFF, // 'H'-'O'
	0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C, // 'P'-'W'
	0x1D, 0x1E, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 'X'-'Z'
	0xFF, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, // 'a'-'g'
	0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14, 0x15, 0xFF, // 'h'-'o'
	0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C, // 'p'-'w'
	0x1D, 0x1E, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 'x'-'z'
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x80
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x88
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x90
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0x98
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xA0
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xA8
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xB0
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xB8
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xC0
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xC8
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xD0
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xD8
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xE0
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xE8
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xF0
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // 0xF8
}

// New creates a ULID with the given Unix millisecond timestamp and entropy
// source. If entropy is nil, the random component will be zero. Use
// [DefaultEntropy] for a safe, concurrent monotonic entropy source.
//
// Passing a [MonotonicReader] as entropy ensures that ULIDs generated within
// the same millisecond are lexicographically sortable.
func New(ms uint64, entropy io.Reader) (id ULID, err error) {
	if err = id.SetTime(ms); err != nil {
		return id, err
	}

	switch e := entropy.(type) {
	case nil:
		return id, nil
	case MonotonicReader:
		err = e.MonotonicRead(ms, id[6:])
	default:
		_, err = io.ReadFull(e, id[6:])
	}

	return id, err
}

// MustNew is like [New] but panics on error.
func MustNew(ms uint64, entropy io.Reader) ULID {
	id, err := New(ms, entropy)
	if err != nil {
		panic(err)
	}
	return id
}

// Make returns a new ULID using the current time and the default
// process-global monotonic entropy source. This is the simplest way
// to generate a ULID. It is safe for concurrent use.
func Make() ULID {
	return MustNew(Now(), defaultEntropy())
}

// Parse decodes a 26-character Crockford Base32 encoded string into a ULID.
// Parsing is case-insensitive. Characters not in the encoding alphabet may
// produce incorrect results. For strict validation, use [ParseStrict].
func Parse(ulid string) (id ULID, err error) {
	return id, parse([]byte(ulid), false, &id)
}

// ParseStrict is like [Parse] but returns [ErrInvalidCharacters] if any
// character in the input is not in the Crockford Base32 alphabet.
func ParseStrict(ulid string) (id ULID, err error) {
	return id, parse([]byte(ulid), true, &id)
}

// ParsePrefixed parses a prefixed ULID string in the format "prefix_ulid".
// It returns both the prefix and the parsed ULID. Parsing of the ULID
// portion is case-insensitive.
func ParsePrefixed(s string) (prefix string, id ULID, err error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			prefix = s[:i]
			if i+1+EncodedSize != len(s) {
				return "", id, ErrDataSize
			}
			err = parse([]byte(s[i+1:]), false, &id)
			return prefix, id, err
		}
	}
	return "", id, ErrInvalidPrefix
}

// MustParse is like [Parse] but panics on error.
func MustParse(ulid string) ULID {
	id, err := Parse(ulid)
	if err != nil {
		panic(err)
	}
	return id
}

// MustParseStrict is like [ParseStrict] but panics on error.
func MustParseStrict(ulid string) ULID {
	id, err := ParseStrict(ulid)
	if err != nil {
		panic(err)
	}
	return id
}

// Now returns the current UTC Unix timestamp in milliseconds.
func Now() uint64 {
	return Timestamp(time.Now())
}

// Timestamp converts a [time.Time] to a Unix millisecond timestamp suitable
// for use with [New].
func Timestamp(t time.Time) uint64 {
	return uint64(t.Unix())*1000 + uint64(t.Nanosecond()/int(time.Millisecond))
}

// Time converts a Unix millisecond timestamp to a [time.Time].
func Time(ms uint64) time.Time {
	return time.Unix(int64(ms/1000), int64((ms%1000)*uint64(time.Millisecond)))
}

// MaxTime returns the maximum Unix timestamp in milliseconds that can be
// encoded in a ULID. This corresponds to the year 10889 AD.
func MaxTime() uint64 {
	return maxTime
}

// String returns the ULID as a 26-character lowercase Crockford Base32 string.
func (id ULID) String() string {
	ulid := make([]byte, EncodedSize)
	_ = id.MarshalTextTo(ulid)
	return string(ulid)
}

// Prefixed returns the ULID as a prefixed string in the format "prefix_ulid".
// The prefix should be a short, lowercase, alphanumeric entity type identifier
// such as "user", "txn", or "evt".
func (id ULID) Prefixed(prefix string) string {
	buf := make([]byte, len(prefix)+1+EncodedSize)
	copy(buf, prefix)
	buf[len(prefix)] = '_'
	_ = id.MarshalTextTo(buf[len(prefix)+1:])
	return string(buf)
}

// Bytes returns a copy of the raw 16-byte ULID data.
func (id ULID) Bytes() []byte {
	b := make([]byte, BinarySize)
	copy(b, id[:])
	return b
}

// Time returns the Unix millisecond timestamp component of the ULID.
func (id ULID) Time() uint64 {
	return uint64(id[5]) |
		uint64(id[4])<<8 |
		uint64(id[3])<<16 |
		uint64(id[2])<<24 |
		uint64(id[1])<<32 |
		uint64(id[0])<<40
}

// Timestamp returns the timestamp component of the ULID as a [time.Time].
func (id ULID) Timestamp() time.Time {
	return Time(id.Time())
}

// Entropy returns a copy of the 10-byte entropy component of the ULID.
func (id ULID) Entropy() []byte {
	e := make([]byte, 10)
	copy(e, id[6:])
	return e
}

// IsZero returns true if the ULID is the zero value.
func (id ULID) IsZero() bool {
	return id == Zero
}

// Compare returns an integer comparing two ULIDs lexicographically.
// The result is 0 if id == other, -1 if id < other, and +1 if id > other.
// This ordering is consistent with the ULID's chronological and string sort
// ordering.
func (id ULID) Compare(other ULID) int {
	for i := 0; i < 16; i++ {
		if id[i] < other[i] {
			return -1
		}
		if id[i] > other[i] {
			return 1
		}
	}
	return 0
}

// SetTime sets the timestamp component of the ULID to the given Unix
// millisecond value. Returns [ErrBigTime] if ms exceeds [MaxTime].
func (id *ULID) SetTime(ms uint64) error {
	if ms > maxTime {
		return ErrBigTime
	}
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)
	return nil
}

// SetEntropy sets the entropy component of the ULID. The input must be
// exactly 10 bytes. Returns [ErrDataSize] if the length is wrong.
func (id *ULID) SetEntropy(e []byte) error {
	if len(e) != 10 {
		return ErrDataSize
	}
	copy(id[6:], e)
	return nil
}

// MarshalBinary implements the [encoding.BinaryMarshaler] interface by
// returning a copy of the ULID as a 16-byte slice.
func (id ULID) MarshalBinary() ([]byte, error) {
	b := make([]byte, BinarySize)
	return b, id.MarshalBinaryTo(b)
}

// MarshalBinaryTo writes the binary representation of the ULID to dst.
// dst must be at least 16 bytes. Returns [ErrBufferSize] if too small.
func (id ULID) MarshalBinaryTo(dst []byte) error {
	if len(dst) < BinarySize {
		return ErrBufferSize
	}
	copy(dst, id[:])
	return nil
}

// UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface.
// The input must be exactly 16 bytes.
func (id *ULID) UnmarshalBinary(data []byte) error {
	if len(data) != BinarySize {
		return ErrDataSize
	}
	copy(id[:], data)
	return nil
}

// MarshalText implements the [encoding.TextMarshaler] interface by returning
// the lowercase Crockford Base32 encoding of the ULID.
func (id ULID) MarshalText() ([]byte, error) {
	dst := make([]byte, EncodedSize)
	return dst, id.MarshalTextTo(dst)
}

// MarshalTextTo writes the lowercase Crockford Base32 encoding of the ULID
// to dst. dst must be at least 26 bytes. Returns [ErrBufferSize] if too small.
//
// The encoding is fully unrolled for maximum performance.
func (id ULID) MarshalTextTo(dst []byte) error {
	if len(dst) < EncodedSize {
		return ErrBufferSize
	}

	// Timestamp (6 bytes -> 10 characters)
	dst[0] = encoding[(id[0]&224)>>5]
	dst[1] = encoding[id[0]&31]
	dst[2] = encoding[(id[1]&248)>>3]
	dst[3] = encoding[((id[1]&7)<<2)|((id[2]&192)>>6)]
	dst[4] = encoding[(id[2]&62)>>1]
	dst[5] = encoding[((id[2]&1)<<4)|((id[3]&240)>>4)]
	dst[6] = encoding[((id[3]&15)<<1)|((id[4]&128)>>7)]
	dst[7] = encoding[(id[4]&124)>>2]
	dst[8] = encoding[((id[4]&3)<<3)|((id[5]&224)>>5)]
	dst[9] = encoding[id[5]&31]

	// Entropy (10 bytes -> 16 characters)
	dst[10] = encoding[(id[6]&248)>>3]
	dst[11] = encoding[((id[6]&7)<<2)|((id[7]&192)>>6)]
	dst[12] = encoding[(id[7]&62)>>1]
	dst[13] = encoding[((id[7]&1)<<4)|((id[8]&240)>>4)]
	dst[14] = encoding[((id[8]&15)<<1)|((id[9]&128)>>7)]
	dst[15] = encoding[(id[9]&124)>>2]
	dst[16] = encoding[((id[9]&3)<<3)|((id[10]&224)>>5)]
	dst[17] = encoding[id[10]&31]
	dst[18] = encoding[(id[11]&248)>>3]
	dst[19] = encoding[((id[11]&7)<<2)|((id[12]&192)>>6)]
	dst[20] = encoding[(id[12]&62)>>1]
	dst[21] = encoding[((id[12]&1)<<4)|((id[13]&240)>>4)]
	dst[22] = encoding[((id[13]&15)<<1)|((id[14]&128)>>7)]
	dst[23] = encoding[(id[14]&124)>>2]
	dst[24] = encoding[((id[14]&3)<<3)|((id[15]&224)>>5)]
	dst[25] = encoding[id[15]&31]

	return nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
// Parsing is case-insensitive.
func (id *ULID) UnmarshalText(v []byte) error {
	return parse(v, false, id)
}

// MarshalJSON implements the [encoding/json.Marshaler] interface by returning
// the ULID as a quoted lowercase Crockford Base32 string.
func (id ULID) MarshalJSON() ([]byte, error) {
	dst := make([]byte, EncodedSize+2)
	dst[0] = '"'
	if err := id.MarshalTextTo(dst[1:]); err != nil {
		return nil, err
	}
	dst[EncodedSize+1] = '"'
	return dst, nil
}

// UnmarshalJSON implements the [encoding/json.Unmarshaler] interface.
func (id *ULID) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return ErrDataSize
	}
	return id.UnmarshalText(data[1 : len(data)-1])
}

// Scan implements the [database/sql.Scanner] interface. It supports scanning
// from string and []byte source values.
func (id *ULID) Scan(src interface{}) error {
	switch x := src.(type) {
	case nil:
		return nil
	case string:
		return id.UnmarshalText([]byte(x))
	case []byte:
		if len(x) == BinarySize {
			return id.UnmarshalBinary(x)
		}
		return id.UnmarshalText(x)
	default:
		return ErrScanValue
	}
}

// Value implements the [database/sql/driver.Valuer] interface, returning
// the ULID as a 16-byte binary value. For string storage, convert with
// [ULID.String] before inserting.
func (id ULID) Value() (driver.Value, error) {
	return id.Bytes(), nil
}

// parse decodes a 26-character Crockford Base32 string into a ULID.
// If strict is true, every character is validated against the encoding alphabet.
func parse(v []byte, strict bool, id *ULID) error {
	if len(v) != EncodedSize {
		return ErrDataSize
	}

	// Check for overflow: the first character can only be 0-7 (3 bits max)
	// because 26 Base32 characters encode 130 bits, but ULID is 128 bits.
	if dec[v[0]] > 7 {
		return ErrOverflow
	}

	if strict {
		for i := 0; i < EncodedSize; i++ {
			if dec[v[i]] == 0xFF {
				return ErrInvalidCharacters
			}
		}
	}

	// Timestamp (10 characters -> 6 bytes)
	id[0] = (dec[v[0]] << 5) | dec[v[1]]
	id[1] = (dec[v[2]] << 3) | (dec[v[3]] >> 2)
	id[2] = (dec[v[3]] << 6) | (dec[v[4]] << 1) | (dec[v[5]] >> 4)
	id[3] = (dec[v[5]] << 4) | (dec[v[6]] >> 1)
	id[4] = (dec[v[6]] << 7) | (dec[v[7]] << 2) | (dec[v[8]] >> 3)
	id[5] = (dec[v[8]] << 5) | dec[v[9]]

	// Entropy (16 characters -> 10 bytes)
	id[6] = (dec[v[10]] << 3) | (dec[v[11]] >> 2)
	id[7] = (dec[v[11]] << 6) | (dec[v[12]] << 1) | (dec[v[13]] >> 4)
	id[8] = (dec[v[13]] << 4) | (dec[v[14]] >> 1)
	id[9] = (dec[v[14]] << 7) | (dec[v[15]] << 2) | (dec[v[16]] >> 3)
	id[10] = (dec[v[16]] << 5) | dec[v[17]]
	id[11] = (dec[v[18]] << 3) | (dec[v[19]] >> 2)
	id[12] = (dec[v[19]] << 6) | (dec[v[20]] << 1) | (dec[v[21]] >> 4)
	id[13] = (dec[v[21]] << 4) | (dec[v[22]] >> 1)
	id[14] = (dec[v[22]] << 7) | (dec[v[23]] << 2) | (dec[v[24]] >> 3)
	id[15] = (dec[v[24]] << 5) | dec[v[25]]

	return nil
}

// defaultEntropy returns the process-global locked monotonic entropy source.
var defaultEntropy = func() func() io.Reader {
	var e io.Reader
	var once = new(sync.Once)
	return func() io.Reader {
		once.Do(func() {
			e = &LockedMonotonicReader{
				MonotonicReader: Monotonic(rand.Reader, 0),
			}
		})
		return e
	}
}()

// DefaultEntropy returns the process-global, thread-safe, monotonic entropy
// source backed by crypto/rand. This is the same entropy source used by [Make].
func DefaultEntropy() io.Reader {
	return defaultEntropy()
}
