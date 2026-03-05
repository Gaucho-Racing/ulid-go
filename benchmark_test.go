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
		for i := 0; i < b.N; i++ {
			ulid.New(ulid.Now(), rand.Reader)
		}
	})

	b.Run("math/rand", func(b *testing.B) {
		rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ulid.New(ulid.Now(), rng)
		}
	})

	b.Run("monotonic/crypto", func(b *testing.B) {
		entropy := ulid.Monotonic(rand.Reader, 0)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ulid.New(ulid.Now(), entropy)
		}
	})

	b.Run("monotonic/math", func(b *testing.B) {
		rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		entropy := ulid.Monotonic(rng, 0)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ulid.New(ulid.Now(), entropy)
		}
	})
}

func BenchmarkMake(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ulid.Make()
	}
}

func BenchmarkParse(b *testing.B) {
	s := ulid.Make().String()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ulid.Parse(s)
	}
}

func BenchmarkParseStrict(b *testing.B) {
	s := ulid.Make().String()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ulid.ParseStrict(s)
	}
}

func BenchmarkParsePrefixed(b *testing.B) {
	s := ulid.Make().Prefixed("user")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ulid.ParsePrefixed(s)
	}
}

func BenchmarkString(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

func BenchmarkPrefixed(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Prefixed("user")
	}
}

func BenchmarkMarshalText(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.MarshalText()
	}
}

func BenchmarkMarshalTextTo(b *testing.B) {
	id := ulid.Make()
	buf := make([]byte, ulid.EncodedSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.MarshalTextTo(buf)
	}
}

func BenchmarkMarshalBinary(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.MarshalBinary()
	}
}

func BenchmarkMarshalBinaryTo(b *testing.B) {
	id := ulid.Make()
	buf := make([]byte, ulid.BinarySize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.MarshalBinaryTo(buf)
	}
}

func BenchmarkUnmarshalText(b *testing.B) {
	text := []byte(ulid.Make().String())
	var id ulid.ULID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.UnmarshalText(text)
	}
}

func BenchmarkUnmarshalBinary(b *testing.B) {
	data := ulid.Make().Bytes()
	var id ulid.ULID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.UnmarshalBinary(data)
	}
}

func BenchmarkCompare(b *testing.B) {
	a := ulid.Make()
	c := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Compare(c)
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.MarshalJSON()
	}
}

func BenchmarkNow(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ulid.Now()
	}
}

func BenchmarkTime(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.Time()
	}
}

func BenchmarkTimestamp(b *testing.B) {
	id := ulid.Make()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.Timestamp()
	}
}

// ------- Generator benchmarks -------

func BenchmarkGeneratorMake(b *testing.B) {
	b.Run("no_node_id", func(b *testing.B) {
		gen := ulid.NewGenerator()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			gen.Make()
		}
	})

	b.Run("with_node_id", func(b *testing.B) {
		gen := ulid.NewGenerator(ulid.WithNodeID(42))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			gen.Make()
		}
	})
}

func BenchmarkGeneratorMakePrefixed(b *testing.B) {
	gen := ulid.NewGenerator(ulid.WithNodeID(1), ulid.WithPrefix("txn"))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.MakePrefixed()
	}
}

func BenchmarkGeneratorConcurrent(b *testing.B) {
	gen := ulid.NewGenerator(ulid.WithNodeID(1))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gen.Make()
		}
	})
}
