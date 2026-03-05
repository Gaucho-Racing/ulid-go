package ulid

import (
	"crypto/rand"
	"io"
	"sync"
)

// Generator produces ULIDs with configurable entropy sources and optional
// node ID partitioning for distributed uniqueness guarantees.
//
// A Generator with a unique node ID guarantees collision-free IDs across
// nodes without any coordination. The node ID occupies the first 2 bytes of
// the 80-bit entropy field (supporting up to 65,536 nodes), leaving 64 bits
// for monotonic random entropy.
//
// A Generator is safe for concurrent use.
type Generator struct {
	mu      sync.Mutex
	entropy MonotonicReader
	nodeID  uint16
	hasNode bool
	prefix  string
}

// GeneratorOption configures a [Generator].
type GeneratorOption func(*Generator)

// WithNodeID sets a node identifier (0-65535) that is embedded in the first
// 2 bytes of the entropy portion. This partitions the entropy space per node,
// guaranteeing that two generators with different node IDs will never produce
// the same ULID, even within the same millisecond, without any external
// coordination.
func WithNodeID(id uint16) GeneratorOption {
	return func(g *Generator) {
		g.nodeID = id
		g.hasNode = true
	}
}

// WithEntropy sets the underlying entropy source for the generator.
// The source will be wrapped in a [MonotonicEntropy] reader. If not set,
// the generator defaults to crypto/rand.
func WithEntropy(r io.Reader) GeneratorOption {
	return func(g *Generator) {
		g.entropy = Monotonic(r, 0)
	}
}

// WithPrefix sets a default prefix for IDs produced by this generator.
// When set, [Generator.MakePrefixed] without arguments will use this prefix.
func WithPrefix(prefix string) GeneratorOption {
	return func(g *Generator) {
		g.prefix = prefix
	}
}

// NewGenerator creates a new [Generator] with the given options.
//
// For distributed systems, use [WithNodeID] to assign a unique node
// identifier to each process or machine:
//
//	gen := ulid.NewGenerator(ulid.WithNodeID(1))
//
// The generator is safe for concurrent use across goroutines.
func NewGenerator(opts ...GeneratorOption) *Generator {
	g := &Generator{}
	for _, opt := range opts {
		opt(g)
	}
	if g.entropy == nil {
		g.entropy = Monotonic(rand.Reader, 0)
	}
	return g
}

// Make generates a new ULID. If the generator was created with [WithNodeID],
// the node ID is embedded in the entropy to guarantee distributed uniqueness.
// This method is safe for concurrent use.
func (g *Generator) Make() ULID {
	g.mu.Lock()
	id, err := New(Now(), g.entropy)
	if err != nil {
		g.mu.Unlock()
		panic(err)
	}
	if g.hasNode {
		id[6] = byte(g.nodeID >> 8)
		id[7] = byte(g.nodeID)
	}
	g.mu.Unlock()
	return id
}

// MakePrefixed generates a new ULID and returns it as a prefixed string.
// If prefix is empty, the generator's default prefix (set via [WithPrefix])
// is used. Panics if no prefix is available.
func (g *Generator) MakePrefixed(prefix ...string) string {
	id := g.Make()
	p := g.prefix
	if len(prefix) > 0 && prefix[0] != "" {
		p = prefix[0]
	}
	if p == "" {
		panic("ulid: no prefix specified")
	}
	return id.Prefixed(p)
}

// New generates a ULID with the given timestamp. If the generator was created
// with [WithNodeID], the node ID is embedded in the entropy.
// This method is safe for concurrent use.
func (g *Generator) New(ms uint64) (ULID, error) {
	g.mu.Lock()
	id, err := New(ms, g.entropy)
	if err != nil {
		g.mu.Unlock()
		return id, err
	}
	if g.hasNode {
		id[6] = byte(g.nodeID >> 8)
		id[7] = byte(g.nodeID)
	}
	g.mu.Unlock()
	return id, nil
}

// NodeID returns the generator's node ID and whether one was configured.
func (g *Generator) NodeID() (uint16, bool) {
	return g.nodeID, g.hasNode
}
