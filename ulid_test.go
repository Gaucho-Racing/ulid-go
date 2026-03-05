package ulid_test

import (
	"bytes"
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	mrand "math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Gaucho-Racing/ulid-go"
)

func TestNew(t *testing.T) {
	t.Run("with entropy", func(t *testing.T) {
		id, err := ulid.New(ulid.Now(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if id.IsZero() {
			t.Fatal("expected non-zero ULID")
		}
	})

	t.Run("with nil entropy", func(t *testing.T) {
		ms := ulid.Now()
		id, err := ulid.New(ms, nil)
		if err != nil {
			t.Fatal(err)
		}
		if id.Time() != ms {
			t.Fatalf("expected timestamp %d, got %d", ms, id.Time())
		}
		// Entropy should be all zeros.
		for _, b := range id.Entropy() {
			if b != 0 {
				t.Fatal("expected zero entropy with nil reader")
			}
		}
	})

	t.Run("with big time", func(t *testing.T) {
		_, err := ulid.New(ulid.MaxTime()+1, nil)
		if err != ulid.ErrBigTime {
			t.Fatalf("expected ErrBigTime, got %v", err)
		}
	})

	t.Run("with max time", func(t *testing.T) {
		id, err := ulid.New(ulid.MaxTime(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if id.Time() != ulid.MaxTime() {
			t.Fatalf("expected max time %d, got %d", ulid.MaxTime(), id.Time())
		}
	})
}

func TestMustNew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		id := ulid.MustNew(ulid.Now(), rand.Reader)
		if id.IsZero() {
			t.Fatal("expected non-zero ULID")
		}
	})

	t.Run("panics on error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic")
			}
		}()
		ulid.MustNew(ulid.MaxTime()+1, nil)
	})
}

func TestMake(t *testing.T) {
	id := ulid.Make()
	if id.IsZero() {
		t.Fatal("expected non-zero ULID from Make()")
	}

	now := ulid.Now()
	ts := id.Time()
	if ts > now || ts < now-1000 {
		t.Fatalf("timestamp %d not within 1s of now %d", ts, now)
	}
}

func TestParse(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		orig := ulid.Make()
		s := orig.String()
		parsed, err := ulid.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if orig != parsed {
			t.Fatalf("expected %v, got %v", orig, parsed)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		orig := ulid.Make()
		s := orig.String()
		lower, err := ulid.Parse(strings.ToLower(s))
		if err != nil {
			t.Fatal(err)
		}
		if orig != lower {
			t.Fatalf("case-insensitive parse failed: expected %v, got %v", orig, lower)
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		_, err := ulid.Parse("short")
		if err != ulid.ErrDataSize {
			t.Fatalf("expected ErrDataSize, got %v", err)
		}
	})

	t.Run("overflow", func(t *testing.T) {
		// First character > '7' overflows 128 bits.
		_, err := ulid.Parse("80000000000000000000000000")
		if err != ulid.ErrOverflow {
			t.Fatalf("expected ErrOverflow, got %v", err)
		}
	})

	t.Run("max valid", func(t *testing.T) {
		id, err := ulid.Parse("7ZZZZZZZZZZZZZZZZZZZZZZZZZ")
		if err != nil {
			t.Fatal(err)
		}
		if id.Time() != ulid.MaxTime() {
			t.Fatalf("expected max time, got %d", id.Time())
		}
	})
}

func TestParseStrict(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		orig := ulid.Make()
		_, err := ulid.ParseStrict(orig.String())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid characters", func(t *testing.T) {
		// 'I' is not in the Crockford Base32 alphabet.
		_, err := ulid.ParseStrict("0000000000000000000000000I")
		if err != ulid.ErrInvalidCharacters {
			t.Fatalf("expected ErrInvalidCharacters, got %v", err)
		}
	})

	t.Run("lowercase valid", func(t *testing.T) {
		orig := ulid.Make()
		_, err := ulid.ParseStrict(strings.ToLower(orig.String()))
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestMustParse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		orig := ulid.Make()
		parsed := ulid.MustParse(orig.String())
		if orig != parsed {
			t.Fatalf("expected %v, got %v", orig, parsed)
		}
	})

	t.Run("panics on error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic")
			}
		}()
		ulid.MustParse("bad")
	})
}

func TestMustParseStrict(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	ulid.MustParseStrict("0000000000000000000000000I")
}

func TestTimestamp(t *testing.T) {
	now := time.Now()
	ms := ulid.Timestamp(now)

	expected := uint64(now.Unix())*1000 + uint64(now.Nanosecond()/int(time.Millisecond))
	if ms != expected {
		t.Fatalf("expected %d, got %d", expected, ms)
	}
}

func TestTime(t *testing.T) {
	ms := uint64(1609459200000) // 2021-01-01 00:00:00 UTC
	tt := ulid.Time(ms)

	if tt.Unix() != 1609459200 {
		t.Fatalf("expected unix 1609459200, got %d", tt.Unix())
	}
}

func TestMaxTime(t *testing.T) {
	expected := uint64((1 << 48) - 1)
	if ulid.MaxTime() != expected {
		t.Fatalf("expected %d, got %d", expected, ulid.MaxTime())
	}
}

func TestTimestampRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	ms := ulid.Timestamp(now)
	recovered := ulid.Time(ms)

	if !now.Equal(recovered) {
		t.Fatalf("timestamp round-trip failed: %v != %v", now, recovered)
	}
}

func TestULIDString(t *testing.T) {
	id := ulid.Make()
	s := id.String()

	if len(s) != ulid.EncodedSize {
		t.Fatalf("expected %d characters, got %d", ulid.EncodedSize, len(s))
	}

	// Verify all characters are in the encoding alphabet.
	for i, c := range s {
		if !strings.ContainsRune(ulid.Encoding, c) {
			t.Fatalf("character %c at position %d not in encoding alphabet", c, i)
		}
	}
}

func TestULIDBytes(t *testing.T) {
	id := ulid.Make()
	b := id.Bytes()

	if len(b) != ulid.BinarySize {
		t.Fatalf("expected %d bytes, got %d", ulid.BinarySize, len(b))
	}

	// Verify Bytes returns a copy, not a reference.
	b[0] = 0xFF
	if id[0] == 0xFF && id.Bytes()[0] == 0xFF {
		t.Fatal("Bytes() should return a copy, not a reference to internal state")
	}
}

func TestULIDTime(t *testing.T) {
	ms := uint64(1234567890123)
	id, err := ulid.New(ms, nil)
	if err != nil {
		t.Fatal(err)
	}

	if id.Time() != ms {
		t.Fatalf("expected %d, got %d", ms, id.Time())
	}
}

func TestULIDEntropy(t *testing.T) {
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	e := id.Entropy()
	if len(e) != 10 {
		t.Fatalf("expected 10 entropy bytes, got %d", len(e))
	}

	// Verify Entropy returns a copy.
	e[0] = 0xFF
	if id[6] == 0xFF && id.Entropy()[0] == 0xFF {
		t.Fatal("Entropy() should return a copy")
	}
}

func TestULIDIsZero(t *testing.T) {
	var zero ulid.ULID
	if !zero.IsZero() {
		t.Fatal("zero value should be zero")
	}

	id := ulid.Make()
	if id.IsZero() {
		t.Fatal("Make() should not produce zero value")
	}
}

func TestULIDCompare(t *testing.T) {
	a, _ := ulid.New(1000, nil)
	b, _ := ulid.New(2000, nil)

	if a.Compare(b) >= 0 {
		t.Fatal("expected a < b")
	}
	if b.Compare(a) <= 0 {
		t.Fatal("expected b > a")
	}
	if a.Compare(a) != 0 {
		t.Fatal("expected a == a")
	}
}

func TestSetTime(t *testing.T) {
	var id ulid.ULID

	if err := id.SetTime(12345); err != nil {
		t.Fatal(err)
	}
	if id.Time() != 12345 {
		t.Fatalf("expected 12345, got %d", id.Time())
	}

	if err := id.SetTime(ulid.MaxTime() + 1); err != ulid.ErrBigTime {
		t.Fatalf("expected ErrBigTime, got %v", err)
	}
}

func TestSetEntropy(t *testing.T) {
	var id ulid.ULID

	e := make([]byte, 10)
	for i := range e {
		e[i] = byte(i + 1)
	}
	if err := id.SetEntropy(e); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(id.Entropy(), e) {
		t.Fatalf("expected %v, got %v", e, id.Entropy())
	}

	if err := id.SetEntropy(make([]byte, 5)); err != ulid.ErrDataSize {
		t.Fatalf("expected ErrDataSize, got %v", err)
	}
}

func TestMarshalBinary(t *testing.T) {
	id := ulid.Make()

	data, err := id.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != ulid.BinarySize {
		t.Fatalf("expected %d bytes, got %d", ulid.BinarySize, len(data))
	}

	var parsed ulid.ULID
	if err := parsed.UnmarshalBinary(data); err != nil {
		t.Fatal(err)
	}
	if id != parsed {
		t.Fatalf("binary round-trip failed: %v != %v", id, parsed)
	}
}

func TestMarshalBinaryTo(t *testing.T) {
	id := ulid.Make()

	t.Run("success", func(t *testing.T) {
		buf := make([]byte, 16)
		if err := id.MarshalBinaryTo(buf); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf, id[:]) {
			t.Fatal("MarshalBinaryTo produced wrong bytes")
		}
	})

	t.Run("buffer too small", func(t *testing.T) {
		buf := make([]byte, 10)
		if err := id.MarshalBinaryTo(buf); err != ulid.ErrBufferSize {
			t.Fatalf("expected ErrBufferSize, got %v", err)
		}
	})
}

func TestUnmarshalBinary(t *testing.T) {
	t.Run("wrong size", func(t *testing.T) {
		var id ulid.ULID
		if err := id.UnmarshalBinary([]byte{1, 2, 3}); err != ulid.ErrDataSize {
			t.Fatalf("expected ErrDataSize, got %v", err)
		}
	})
}

func TestMarshalText(t *testing.T) {
	id := ulid.Make()

	data, err := id.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != ulid.EncodedSize {
		t.Fatalf("expected %d bytes, got %d", ulid.EncodedSize, len(data))
	}
	if string(data) != id.String() {
		t.Fatalf("MarshalText disagrees with String: %q != %q", data, id.String())
	}

	var parsed ulid.ULID
	if err := parsed.UnmarshalText(data); err != nil {
		t.Fatal(err)
	}
	if id != parsed {
		t.Fatalf("text round-trip failed: %v != %v", id, parsed)
	}
}

func TestMarshalTextTo(t *testing.T) {
	id := ulid.Make()

	t.Run("buffer too small", func(t *testing.T) {
		buf := make([]byte, 10)
		if err := id.MarshalTextTo(buf); err != ulid.ErrBufferSize {
			t.Fatalf("expected ErrBufferSize, got %v", err)
		}
	})
}

func TestJSON(t *testing.T) {
	type wrapper struct {
		ID ulid.ULID `json:"id"`
	}

	id := ulid.Make()
	w := wrapper{ID: id}

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}

	expected := fmt.Sprintf(`{"id":"%s"}`, id.String())
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, data)
	}

	var parsed wrapper
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if w.ID != parsed.ID {
		t.Fatalf("JSON round-trip failed: %v != %v", w.ID, parsed.ID)
	}
}

func TestJSONMarshal(t *testing.T) {
	id := ulid.Make()
	data, err := id.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	expected := fmt.Sprintf(`"%s"`, id.String())
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, data)
	}
}

func TestJSONUnmarshalErrors(t *testing.T) {
	var id ulid.ULID

	// No quotes.
	if err := id.UnmarshalJSON([]byte(`notquoted`)); err != ulid.ErrDataSize {
		t.Fatalf("expected ErrDataSize, got %v", err)
	}

	// Empty quotes.
	if err := id.UnmarshalJSON([]byte(`""`)); err != ulid.ErrDataSize {
		t.Fatalf("expected ErrDataSize for empty string, got %v", err)
	}
}

func TestScan(t *testing.T) {
	id := ulid.Make()

	t.Run("from string", func(t *testing.T) {
		var scanned ulid.ULID
		if err := scanned.Scan(id.String()); err != nil {
			t.Fatal(err)
		}
		if id != scanned {
			t.Fatalf("Scan from string failed: %v != %v", id, scanned)
		}
	})

	t.Run("from bytes (binary)", func(t *testing.T) {
		var scanned ulid.ULID
		if err := scanned.Scan(id.Bytes()); err != nil {
			t.Fatal(err)
		}
		if id != scanned {
			t.Fatalf("Scan from bytes failed: %v != %v", id, scanned)
		}
	})

	t.Run("from bytes (text)", func(t *testing.T) {
		var scanned ulid.ULID
		if err := scanned.Scan([]byte(id.String())); err != nil {
			t.Fatal(err)
		}
		if id != scanned {
			t.Fatalf("Scan from text bytes failed: %v != %v", id, scanned)
		}
	})

	t.Run("from nil", func(t *testing.T) {
		var scanned ulid.ULID
		if err := scanned.Scan(nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		var scanned ulid.ULID
		if err := scanned.Scan(12345); err != ulid.ErrScanValue {
			t.Fatalf("expected ErrScanValue, got %v", err)
		}
	})
}

func TestValue(t *testing.T) {
	id := ulid.Make()
	v, err := id.Value()
	if err != nil {
		t.Fatal(err)
	}

	b, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", v)
	}
	if !bytes.Equal(b, id.Bytes()) {
		t.Fatal("Value() bytes don't match Bytes()")
	}

	// Verify it implements driver.Valuer.
	var _ driver.Valuer = id
}

func TestEncodingRoundTrip(t *testing.T) {
	// Generate many ULIDs and verify they survive encoding round-trips.
	for i := 0; i < 1000; i++ {
		orig := ulid.Make()

		// Text round-trip.
		s := orig.String()
		parsed, err := ulid.Parse(s)
		if err != nil {
			t.Fatalf("iteration %d: Parse failed: %v", i, err)
		}
		if orig != parsed {
			t.Fatalf("iteration %d: text round-trip failed", i)
		}

		// Binary round-trip.
		data, _ := orig.MarshalBinary()
		var bin ulid.ULID
		if err := bin.UnmarshalBinary(data); err != nil {
			t.Fatalf("iteration %d: UnmarshalBinary failed: %v", i, err)
		}
		if orig != bin {
			t.Fatalf("iteration %d: binary round-trip failed", i)
		}
	}
}

func TestLexicographicSortOrder(t *testing.T) {
	ids := make([]ulid.ULID, 100)
	for i := range ids {
		ids[i] = ulid.Make()
	}

	// String sort order should match Compare order.
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i].Compare(ids[j]) < 0
	})
	sort.Strings(strs)

	for i, id := range ids {
		if id.String() != strs[i] {
			t.Fatalf("lexicographic sort mismatch at index %d: Compare gives %s, string sort gives %s",
				i, id.String(), strs[i])
		}
	}
}

func TestMonotonicSortOrder(t *testing.T) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	ms := ulid.Now()

	prev, err := ulid.New(ms, entropy)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 1000; i++ {
		next, err := ulid.New(ms, entropy)
		if err != nil {
			t.Fatal(err)
		}
		if next.Compare(prev) <= 0 {
			t.Fatalf("iteration %d: monotonic order violated: %s >= %s",
				i, prev.String(), next.String())
		}
		prev = next
	}
}

func TestMonotonicNewMillisecond(t *testing.T) {
	entropy := ulid.Monotonic(rand.Reader, 0)

	id1, err := ulid.New(1000, entropy)
	if err != nil {
		t.Fatal(err)
	}

	id2, err := ulid.New(2000, entropy)
	if err != nil {
		t.Fatal(err)
	}

	// Different milliseconds should produce different ULIDs.
	if id1 == id2 {
		t.Fatal("different timestamps should produce different ULIDs")
	}

	// The second should be greater.
	if id2.Compare(id1) <= 0 {
		t.Fatal("later timestamp should produce greater ULID")
	}
}

func TestMonotonicOverflow(t *testing.T) {
	// Create a ULID with max entropy and verify that incrementing overflows.
	// We need 10 bytes for the initial entropy read, plus 8 bytes for the
	// increment read (since bytes.Reader doesn't implement the rng fast path).
	maxEntropy := bytes.NewReader([]byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // initial entropy (all max)
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x01, // increment data (will produce inc >= 1)
	})

	entropy := ulid.Monotonic(maxEntropy, 1)
	ms := ulid.Now()

	// First call sets the entropy to all 0xFF.
	_, err := ulid.New(ms, entropy)
	if err != nil {
		t.Fatal(err)
	}

	// Second call within same ms should overflow since entropy is at max.
	_, err = ulid.New(ms, entropy)
	if err != ulid.ErrMonotonicOverflow {
		t.Fatalf("expected ErrMonotonicOverflow, got %v", err)
	}
}

func TestConcurrentMake(t *testing.T) {
	var wg sync.WaitGroup
	ids := make([]ulid.ULID, 1000)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = ulid.Make()
		}(i)
	}
	wg.Wait()

	// All IDs should be unique.
	seen := make(map[ulid.ULID]bool)
	for _, id := range ids {
		if id.IsZero() {
			t.Fatal("got zero ULID from concurrent Make()")
		}
		if seen[id] {
			t.Fatalf("duplicate ULID from concurrent Make(): %s", id.String())
		}
		seen[id] = true
	}
}

func TestZeroULID(t *testing.T) {
	s := ulid.Zero.String()
	expected := "00000000000000000000000000"
	if s != expected {
		t.Fatalf("expected %s, got %s", expected, s)
	}

	parsed, err := ulid.Parse(expected)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != ulid.Zero {
		t.Fatal("parsed zero string should equal Zero")
	}
}

func TestKnownValues(t *testing.T) {
	// Test against known ULID values to verify encoding correctness.

	// A ULID with timestamp 0 and entropy 0 should be all zeros.
	id, _ := ulid.New(0, nil)
	if id.String() != "00000000000000000000000000" {
		t.Fatalf("zero ULID string: expected 00000000000000000000000000, got %s", id.String())
	}

	// Max ULID.
	maxID, err := ulid.Parse("7ZZZZZZZZZZZZZZZZZZZZZZZZZ")
	if err != nil {
		t.Fatal(err)
	}
	if maxID.Time() != ulid.MaxTime() {
		t.Fatalf("max ULID time: expected %d, got %d", ulid.MaxTime(), maxID.Time())
	}
}

func TestDefaultEntropy(t *testing.T) {
	e := ulid.DefaultEntropy()
	if e == nil {
		t.Fatal("DefaultEntropy() returned nil")
	}

	id, err := ulid.New(ulid.Now(), e)
	if err != nil {
		t.Fatal(err)
	}
	if id.IsZero() {
		t.Fatal("ULID from DefaultEntropy should not be zero")
	}
}

func TestMonotonicWithMathRand(t *testing.T) {
	// math/rand.Rand implements the rng interface (Int63n), which should
	// trigger the fast-path increment.
	source := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	entropy := ulid.Monotonic(source, 0)
	ms := ulid.Now()

	prev, err := ulid.New(ms, entropy)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		next, err := ulid.New(ms, entropy)
		if err != nil {
			t.Fatal(err)
		}
		if next.Compare(prev) <= 0 {
			t.Fatalf("monotonic order violated with math/rand at iteration %d", i)
		}
		prev = next
	}
}

func TestEncodingAlphabet(t *testing.T) {
	// Verify the encoding alphabet has exactly 32 unique characters.
	if len(ulid.Encoding) != 32 {
		t.Fatalf("expected 32 characters, got %d", len(ulid.Encoding))
	}

	seen := make(map[rune]bool)
	for _, c := range ulid.Encoding {
		if seen[c] {
			t.Fatalf("duplicate character in encoding: %c", c)
		}
		seen[c] = true
	}

	// Verify excluded characters.
	for _, c := range "IiLlOoUu" {
		if strings.ContainsRune(ulid.Encoding, c) {
			t.Fatalf("encoding should not contain %c", c)
		}
	}
}

func TestTimestampPreservation(t *testing.T) {
	// Verify various timestamps survive encode/decode.
	timestamps := []uint64{
		0,
		1,
		1000,
		uint64(time.Now().UnixMilli()),
		ulid.MaxTime() - 1,
		ulid.MaxTime(),
	}

	for _, ms := range timestamps {
		id, err := ulid.New(ms, rand.Reader)
		if err != nil {
			t.Fatalf("New(%d): %v", ms, err)
		}

		if id.Time() != ms {
			t.Fatalf("timestamp %d not preserved: got %d", ms, id.Time())
		}

		// Verify through string round-trip.
		parsed, err := ulid.Parse(id.String())
		if err != nil {
			t.Fatalf("Parse failed for timestamp %d: %v", ms, err)
		}
		if parsed.Time() != ms {
			t.Fatalf("timestamp %d not preserved after text round-trip: got %d", ms, parsed.Time())
		}
	}
}

func TestLockedMonotonicReader(t *testing.T) {
	inner := ulid.Monotonic(rand.Reader, 0)
	locked := &ulid.LockedMonotonicReader{MonotonicReader: inner}

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	ms := ulid.Now()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ulid.New(ms, locked)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent monotonic read error: %v", err)
	}
}

func TestMonotonicSmallIncrement(t *testing.T) {
	// With inc=1, we should get exactly +1 increments.
	entropy := ulid.Monotonic(rand.Reader, 1)
	ms := ulid.Now()

	prev, err := ulid.New(ms, entropy)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		next, err := ulid.New(ms, entropy)
		if err != nil {
			t.Fatal(err)
		}
		if next.Compare(prev) <= 0 {
			t.Fatalf("iteration %d: monotonic order violated", i)
		}
		prev = next
	}
}

func TestOverflowBoundary(t *testing.T) {
	// Characters 0-7 in first position should be valid.
	for c := byte('0'); c <= '7'; c++ {
		s := string(c) + strings.Repeat("0", 25)
		if _, err := ulid.Parse(s); err != nil {
			t.Fatalf("character %c should be valid in first position: %v", c, err)
		}
	}

	// Characters 8-Z in first position should overflow.
	overflow := []string{
		"80000000000000000000000000",
		"90000000000000000000000000",
		"A0000000000000000000000000",
		"G0000000000000000000000000",
		"Z0000000000000000000000000",
	}
	for _, s := range overflow {
		if _, err := ulid.Parse(s); err != ulid.ErrOverflow {
			t.Fatalf("string %q should overflow, got err=%v", s, err)
		}
	}
}

// Verify we don't panic with math.MaxUint64 as increment.
func TestMonotonicLargeIncrement(t *testing.T) {
	entropy := ulid.Monotonic(rand.Reader, math.MaxUint64)
	ms := ulid.Now()

	_, err := ulid.New(ms, entropy)
	if err != nil {
		t.Fatal(err)
	}

	// Second call may or may not overflow depending on random entropy,
	// but it should not panic.
	_, _ = ulid.New(ms, entropy)
}
