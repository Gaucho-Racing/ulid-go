# ulid-go

[![CI](https://github.com/Gaucho-Racing/ulid-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Gaucho-Racing/ulid-go/actions/workflows/ci.yml)

A blazing fast, production-grade [ULID](https://github.com/ulid/spec) implementation in Go with lowercase output, prefix support, and distributed collision-free guarantees.

```
  01an4z07by      79ka1307sr9x4mv3

 |----------|    |----------------|
  Timestamp          Randomness
   48 bits            80 bits
```

## Features

- **Lowercase by default** — all string output uses lowercase Crockford Base32
- **Prefix support** — generate entity-scoped IDs like `user_01arz3ndek...` or `txn_01arz3ndek...`
- **Distributed uniqueness** — `Generator` with node ID partitioning guarantees collision-free IDs across up to 65,536 nodes without coordination
- **Monotonic sorting** — IDs generated within the same millisecond are strictly ordered
- **Zero-allocation hot paths** — `Parse`, `MarshalTextTo`, `MarshalBinaryTo`, `UnmarshalText`, `UnmarshalBinary` allocate nothing
- **Fully unrolled encoding** — Crockford Base32 encode/decode with no loops
- **Thread-safe** — `Make()`, `Generator`, and `DefaultEntropy()` are safe for concurrent use
- **128-bit UUID compatible** — drop-in replacement for UUID columns in databases
- **Standard interfaces** — implements `encoding.TextMarshaler`, `encoding.BinaryMarshaler`, `json.Marshaler`, `sql.Scanner`, `driver.Valuer`, `fmt.Stringer`

## Getting Started

### Installing

With [Go's module support](https://go.dev/wiki/Modules#how-to-use-modules), `go [build|run|test]` automatically fetches the necessary dependencies when you add the import in your code:

```go
import "github.com/Gaucho-Racing/ulid-go"
```

Alternatively, use `go get`:

```sh
go get -u github.com/Gaucho-Racing/ulid-go
```

### Usage

```go
package main

import (
    "fmt"
    "github.com/Gaucho-Racing/ulid-go"
)

func main() {
    // Generate a ULID
    id := ulid.Make()
    fmt.Println(id) // 01jgy5fz7rqv8s3n0x4m6k2w1h

    // With a prefix
    fmt.Println(id.Prefixed("user")) // user_01jgy5fz7rqv8s3n0x4m6k2w1h

    // Parse it back
    parsed, _ := ulid.Parse("01jgy5fz7rqv8s3n0x4m6k2w1h")
    fmt.Println(parsed.Time())       // Unix millisecond timestamp
    fmt.Println(parsed.Timestamp())  // time.Time

    // Parse prefixed IDs
    prefix, id, _ := ulid.ParsePrefixed("user_01jgy5fz7rqv8s3n0x4m6k2w1h")
    fmt.Println(prefix) // "user"
}
```

### Distributed Systems

For multi-node deployments, use a `Generator` with a unique node ID per process. The node ID is embedded in the entropy field, partitioning the ID space so that no two nodes can ever produce the same ULID — even within the same millisecond — without any external coordination.

```go
// Assign a unique node ID to each process (0-65535)
gen := ulid.NewGenerator(
    ulid.WithNodeID(1),        // unique per node
    ulid.WithPrefix("evt"),    // optional default prefix
)

id := gen.Make()                  // 01jgy5fz7r...
prefixed := gen.MakePrefixed()    // uses default prefix: evt_01jgy5fz7r...
custom := gen.MakePrefixed("txn") // override with "txn": txn_01jgy5fz7r...
```

The node ID occupies the first 2 bytes of the 80-bit entropy field (16 bits = 65,536 possible nodes), leaving 64 bits for monotonic random entropy. Within a single node, monotonic entropy ensures strict ordering and uniqueness. Across nodes, the node ID partition guarantees no collisions.

## API

### Constructors

| Function | Description |
|---|---|
| `Make()` | Generate a ULID with current time and default entropy. Thread-safe. |
| `New(ms, entropy)` | Generate with explicit timestamp and entropy source. |
| `MustNew(ms, entropy)` | Like `New` but panics on error. |
| `Parse(s)` | Decode a 26-char Base32 string. Case-insensitive. |
| `ParseStrict(s)` | Like `Parse` with character validation. |
| `ParsePrefixed(s)` | Parse a `prefix_ulid` string, returning both parts. |
| `MustParse(s)` | Like `Parse` but panics on error. |
| `MustParseStrict(s)` | Like `ParseStrict` but panics on error. |

### ULID Methods

| Method | Description |
|---|---|
| `String()` | 26-char lowercase Crockford Base32 string. |
| `Prefixed(p)` | Prefixed string: `p_<ulid>`. |
| `Bytes()` | Copy of the raw 16-byte data. |
| `Time()` | Unix millisecond timestamp. |
| `Timestamp()` | Timestamp as `time.Time`. |
| `Entropy()` | Copy of the 10-byte entropy. |
| `IsZero()` | True if zero value. |
| `Compare(other)` | Lexicographic comparison (-1, 0, +1). |
| `SetTime(ms)` | Set the timestamp component. |
| `SetEntropy(e)` | Set the entropy component (10 bytes). |

### Serialization

| Interface | Method | Allocations |
|---|---|---|
| `encoding.TextMarshaler` | `MarshalText()` | 1 (32 B) |
| `encoding.TextMarshaler` | `MarshalTextTo(dst)` | 0 |
| `encoding.TextUnmarshaler` | `UnmarshalText(v)` | 0 |
| `encoding.BinaryMarshaler` | `MarshalBinary()` | 1 (16 B) |
| `encoding.BinaryMarshaler` | `MarshalBinaryTo(dst)` | 0 |
| `encoding.BinaryUnmarshaler` | `UnmarshalBinary(data)` | 0 |
| `json.Marshaler` | `MarshalJSON()` | 1 (32 B) |
| `json.Unmarshaler` | `UnmarshalJSON(data)` | 0 |
| `sql.Scanner` | `Scan(src)` | 0 |
| `driver.Valuer` | `Value()` | 1 (16 B) |

### Time Helpers

| Function | Description |
|---|---|
| `Now()` | Current UTC Unix milliseconds. |
| `Timestamp(t)` | Convert `time.Time` to Unix ms. |
| `Time(ms)` | Convert Unix ms to `time.Time`. |
| `MaxTime()` | Maximum encodable timestamp (year 10889). |

### Entropy

| Function | Description |
|---|---|
| `DefaultEntropy()` | Process-global thread-safe monotonic entropy (crypto/rand). |
| `Monotonic(r, inc)` | Create a monotonic entropy source wrapping any `io.Reader`. |

### Generator

| Method | Description |
|---|---|
| `NewGenerator(opts...)` | Create a generator with options. |
| `WithNodeID(id)` | Embed a 16-bit node ID for distributed uniqueness. |
| `WithEntropy(r)` | Use a custom entropy source. |
| `WithPrefix(p)` | Set a default prefix. |
| `gen.Make()` | Generate a ULID. Thread-safe. |
| `gen.MakePrefixed(p...)` | Generate a prefixed ULID string. |
| `gen.New(ms)` | Generate with explicit timestamp. |
| `gen.NodeID()` | Get the configured node ID. |

## Benchmarks

Measured on Apple M1 Max:

```
BenchmarkNew/crypto/rand          6,975,308      159.9 ns/op    16 B/op    1 allocs/op
BenchmarkNew/math/rand           15,504,117       81.73 ns/op   16 B/op    1 allocs/op
BenchmarkNew/monotonic/crypto    16,502,566       72.44 ns/op   16 B/op    1 allocs/op
BenchmarkNew/monotonic/math      18,158,854       65.99 ns/op   16 B/op    1 allocs/op
BenchmarkMake                    14,831,469       81.97 ns/op   16 B/op    1 allocs/op
BenchmarkParse                  100,000,000       10.61 ns/op    0 B/op    0 allocs/op
BenchmarkParseStrict             68,122,470       17.68 ns/op    0 B/op    0 allocs/op
BenchmarkParsePrefixed           88,345,999       13.54 ns/op    0 B/op    0 allocs/op
BenchmarkString                  49,137,532       24.49 ns/op   32 B/op    1 allocs/op
BenchmarkPrefixed                46,420,616       25.32 ns/op   32 B/op    1 allocs/op
BenchmarkMarshalTextTo          100,000,000       10.73 ns/op    0 B/op    0 allocs/op
BenchmarkMarshalBinaryTo        579,008,643        2.19 ns/op    0 B/op    0 allocs/op
BenchmarkUnmarshalText          122,359,550        9.93 ns/op    0 B/op    0 allocs/op
BenchmarkUnmarshalBinary        571,956,039        2.12 ns/op    0 B/op    0 allocs/op
BenchmarkCompare                123,713,810        9.90 ns/op    0 B/op    0 allocs/op
BenchmarkGeneratorMake           14,063,845       88.94 ns/op   16 B/op    1 allocs/op
BenchmarkGeneratorConcurrent      4,958,121      241.0  ns/op   16 B/op    1 allocs/op
```

Key highlights:
- **Parse: 10.6 ns, 0 allocs** — zero-allocation decoding
- **MarshalBinaryTo: 2.2 ns, 0 allocs** — sub-3ns binary serialization
- **UnmarshalBinary: 2.1 ns, 0 allocs** — sub-3ns binary deserialization
- **Make: 82 ns** — generate ~12 million ULIDs/sec per core
- **Generator concurrent: 241 ns** — ~4.1 million ULIDs/sec under contention

## Specification

This library implements the [ULID spec](https://github.com/ulid/spec) with the following layout:

```
 0                   1                   2                   3
  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                      32_bit_uint_time_high                    |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     16_bit_uint_time_low      |       16_bit_uint_random      |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                       32_bit_uint_random                      |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                       32_bit_uint_random                      |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

- **Timestamp**: 48-bit Unix milliseconds (bytes 0-5), valid until year 10889
- **Entropy**: 80-bit random (bytes 6-15), cryptographically sourced by default
- **Encoding**: Crockford Base32, lowercase, 26 characters
- **Monotonicity**: Within the same millisecond, entropy is incremented to ensure strict sort order

When using a `Generator` with a node ID, the entropy layout becomes:

```
 Bytes [6:8]  - 16-bit node ID
 Bytes [8:16] - 64-bit monotonic random entropy
```

## Contributing

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b gh-username/my-amazing-feature`)
3. Commit your Changes (`git commit -m 'Add my amazing feature'`)
4. Push to the Branch (`git push origin gh-username/my-amazing-feature`)
5. Open a Pull Request

## License

MIT — see [LICENSE](LICENSE).
