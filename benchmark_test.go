package ulid_test

import (
	"crypto/rand"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/Gaucho-Racing/ulid-go"
)

func BenchmarkNew(b *testing.B) {
	b.Run("crypto/rand", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			ulid.New(ulid.Now(), rand.Reader)
		}
	})

	b.Run("math/rand", func(b *testing.B) {
		rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		b.ReportAllocs()
		for b.Loop() {
			ulid.New(ulid.Now(), rng)
		}
	})

	b.Run("monotonic/crypto", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.Reader, 0)
		b.ReportAllocs()
		for b.Loop() {
			ulid.New(ulid.Now(), entropy)
		}
	})

	b.Run("monotonic/math", func(b *testing.B) {
		rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		entropy := ulid.Monotonic(rng, 0)
		b.ReportAllocs()
		for b.Loop() {
			ulid.New(ulid.Now(), entropy)
		}
	})
}

func BenchmarkMake(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		ulid.Make()
	}
}

func BenchmarkParse(b *testing.B) {
	s := ulid.Make().String()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ulid.Parse(s)
	}
}

func BenchmarkParseStrict(b *testing.B) {
	s := ulid.Make().String()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ulid.ParseStrict(s)
	}
}

func BenchmarkString(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = id.String()
	}
}

func BenchmarkMarshalText(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.MarshalText()
	}
}

func BenchmarkMarshalTextTo(b *testing.B) {
	id := ulid.Make()
	buf := make([]byte, ulid.EncodedSize)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.MarshalTextTo(buf)
	}
}

func BenchmarkMarshalBinary(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.MarshalBinary()
	}
}

func BenchmarkMarshalBinaryTo(b *testing.B) {
	id := ulid.Make()
	buf := make([]byte, ulid.BinarySize)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.MarshalBinaryTo(buf)
	}
}

func BenchmarkUnmarshalText(b *testing.B) {
	text := []byte(ulid.Make().String())
	var id ulid.ULID
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.UnmarshalText(text)
	}
}

func BenchmarkUnmarshalBinary(b *testing.B) {
	data := ulid.Make().Bytes()
	var id ulid.ULID
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.UnmarshalBinary(data)
	}
}

func BenchmarkCompare(b *testing.B) {
	a := ulid.Make()
	c := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.Compare(c)
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.MarshalJSON()
	}
}

func BenchmarkNow(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		ulid.Now()
	}
}

func BenchmarkTime(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.Time()
	}
}

func BenchmarkTimestamp(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		id.Timestamp()
	}
}
