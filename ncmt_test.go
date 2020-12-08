package ncmt

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/lazyledger/nmt/namespace"
	"github.com/stretchr/testify/assert"
)

// TestRoot checks that the size of each layer
func TestRoot(t *testing.T) {
	// make a tree with 128 leaves of 256 bytes
	leafCount := 128
	tree := mockTree(leafCount, 256, t)

	// check the size of each layer
	for i, l := range tree.layers {
		assert.Equal(t, (leafCount / powerInt(2, i+1)), len(l))
	}

	// check namespace range of the root
	hash := tree.Root()
	// the first namespace.IDSize bits should be the lowest namespace
	assert.Equal(t, hash[0:8], []byte{0, 0, 0, 0, 0, 0, 0, 0})
	// the second 8 bytes should by the 127th namespace
	assert.Equal(t, hash[8:16], []byte{0, 0, 0, 0, 0, 0, 0, 127})
}

func TestConsolidation(t *testing.T) {
	tree := mockTree(16, 4, t)
	// ensure that the namespaces are preserved
	assert.Equal(t, namespace.ID([]byte{0, 0, 0, 0, 0, 0, 0, 3}), tree.layers[1][0].max)
	assert.Equal(t, namespace.ID([]byte{0, 0, 0, 0, 0, 0, 0, 0}), tree.layers[1][0].min)
	assert.Equal(t, namespace.ID([]byte{0, 0, 0, 0, 0, 0, 0, 7}), tree.layers[2][0].max)
	assert.Equal(t, namespace.ID([]byte{0, 0, 0, 0, 0, 0, 0, 0}), tree.layers[2][0].min)
}

func TestLeavesExtension(t *testing.T) {
	data := [][]byte{
		{1, 1}, {2, 2}, {3, 3}, {4, 4},
	}
	lvs := make(leaves, len(data))
	for i, d := range data {
		prefixed := namespace.NewPrefixedData(namespace.IDSize(1), d)
		lvs[i] = newLeaf(sha256.New(), prefixed)
	}
	codec := newRSFG8()
	extended, err := lvs.extend(codec)
	if err != nil {
		t.Error(err)
	}
	// check that the prefixes are the same
	for i, leaf := range extended {
		assert.Equal(t, lvs[i].data.NamespaceID(), leaf.data.NamespaceID())
	}
	// check that the slices have not overlapped
	for i, leaf := range extended {
		assert.NotEqual(t, lvs[i].data.Data(), leaf.data.Data())
	}
	// check that data was erasured as expected
	assert.Equal(t, extended.raw(), [][]byte{{135}, {46}, {26}, {191}})
}

func TestLayerExtension(t *testing.T) {
	layer := make(layer, 4)
	for i := 0; i < 4; i++ {
		layer[i] = node{hash: []byte{byte(i + 1)}}
	}
	codec := newRSFG8()
	extended, err := layer.extend(codec)
	if err != nil {
		t.Error(err)
	}
	// check that the namespace ranges are the same
	for i := range extended {
		assert.Equal(t, layer[i].min, extended[i].min)
		assert.Equal(t, layer[i].max, extended[i].max)
	}

	// check that data was erasured as expected
	assert.Equal(t, extended.raw(), [][]byte{{135}, {46}, {26}, {191}})
}

// create a tree from random data
func mockTree(leafCount, leafSize int, t *testing.T) *NCMT {
	mockData := mockData(leafCount, leafSize)
	tree := NewNCMT()
	for _, d := range mockData {
		err := tree.Push(d)
		if err != nil {
			t.Error(err)
		}
	}
	_, err := tree.Build()
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// create random namespaced data of a set size
func mockData(count, size int) []namespace.Data {
	var output []namespace.Data
	ids := mockIDs(count, 8)
	for i := 0; i < count; i++ {
		// create random data
		rawData := make([]byte, size)
		_, err := rand.Read(rawData)
		if err != nil {
			panic(err)
		}
		id := ids[i]
		data := namespace.NewPrefixedData(id.Size(), append(id, rawData...))
		output = append(output, data)
	}
	return output
}

// creates a slice of increasing order namespace.IDs with max size of size
func mockIDs(count, size int) []namespace.ID {
	out := make([]namespace.ID, count)
	for i := 0; i < count; i++ {
		out[i] = mockID(i)
	}
	return out
}

// creates up to 256 different namespaces
func mockID(id int) namespace.ID {
	switch {
	case id < 256:
		return namespace.ID{0, 0, 0, 0, 0, 0, 0, byte(id)}
	default:
		return namespace.ID{0, 0, 0, 0, 0, 0, 100, 0}
	}
}

// check for errors when building the tree

func powerInt(x, y int) int {
	out := 1
	for y != 0 {
		out *= x
		y--
	}
	return out
}
