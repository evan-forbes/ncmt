package ncmt

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"

	"github.com/lazyledger/nmt/namespace"
)

// Options configure a namespaced coded merkle tree
type Options struct {
	UniformParityNamespace bool
	BatchSize              int
	NamespaceSize          namespace.IDSize
	FreshHash              func() hash.Hash
	Codec                  Codec
}

// Option configures Options.
type Option func(*Options)

// NCMT creates and configures a namespaced coded merkle tree.
type NCMT struct {
	// keep extensions seperate for simplicity
	layers          []layer
	extendedLayers  []layer
	leaves          leaves
	namespaceRanges map[string]leafRange

	originalWidth uint
	// options
	opts *Options
}

// NewNCMT issues a new NCMT using the default options and provided overides
func NewNCMT(setters ...Option) *NCMT {
	defaultOpts := &Options{
		UniformParityNamespace: true,
		BatchSize:              4,
		NamespaceSize:          namespace.IDSize(8),
		FreshHash:              sha256.New,
		Codec:                  RSFG8{},
	}
	for _, setter := range setters {
		setter(defaultOpts)
	}
	return &NCMT{
		namespaceRanges: make(map[string]leafRange),
		opts:            defaultOpts,
	}
}

// Root returns the root hash of the tree. If n.Build has not been called, then
// an empty hash is returned
func (n *NCMT) Root() []byte {
	// return an empty hash if the tree is empty
	if len(n.layers) == 0 {
		return n.opts.FreshHash().Sum(nil)
	}
	latest := n.layers[len(n.layers)-1]
	// return an empty slice for only a partially built tree
	if len(latest) != 1 {
		return []byte{}
	}
	return latest[0].hash
}

/////////////////////////////////////////
// 	Adding Data to the Tree
///////////////////////////////////////

// Push adds data to the leaves of the tree and updates the range. Throws error if data is not pushed
// in order from the lowest (lexographical) id to the greatest
func (n *NCMT) Push(data namespace.Data) error {
	// make sure that the id size is identical across the tree
	if data.NamespaceID().Size() != n.opts.NamespaceSize {
		return fmt.Errorf(
			"invalid push: expected namespaced ID of size %d, received size %d",
			n.opts.NamespaceSize,
			data.NamespaceID(),
		)
	}
	if len(n.leaves) == 0 {
		// add first leaf
		n.leaves = append(n.leaves, newLeaf(n.opts.FreshHash(), data))
		n.updateNamespaceRanges()
		return nil
	}

	// check if new data is being pushed in order (least to greatest)
	lastLeafID := n.leaves[len(n.leaves)-1].data.NamespaceID()
	valid := lastLeafID.LessOrEqual(data.NamespaceID())
	if !valid {
		return errors.New("invalid push: greater or equal namespace.ID required")
	}

	// add the data to existing leaves
	n.leaves = append(n.leaves, newLeaf(n.opts.FreshHash(), data))
	n.updateNamespaceRanges()
	return nil
}

func (n *NCMT) updateNamespaceRanges() {
	if len(n.leaves) > 0 {
		lastIndex := len(n.leaves) - 1
		lastPushed := n.leaves[lastIndex]
		lastNsStr := string(lastPushed.data.NamespaceID())
		lastRange, found := n.namespaceRanges[lastNsStr]
		if !found {
			n.namespaceRanges[lastNsStr] = leafRange{
				start: uint(lastIndex),
				end:   uint(lastIndex + 1),
			}
		} else {
			n.namespaceRanges[lastNsStr] = leafRange{
				start: lastRange.start,
				end:   lastRange.end + 1,
			}
		}
	}
}

// foundInRange check is the range
func (n *NCMT) foundInRange(nID namespace.ID) (bool, uint, uint) {
	foundRng, found := n.namespaceRanges[string(nID)]
	return found, foundRng.start, foundRng.end
}

// A leafRange represents the contiguous set of leaves [Start,End).
type leafRange struct {
	start uint
	end   uint
}

/////////////////////////////////////////
//  Growing the tree
///////////////////////////////////////

// Build recursively consolidates, erasures, and hashes existing leaves until
// the root hash of the tree is generated. Build overides any data cached from a
// previous Build
func (n *NCMT) Build() ([]byte, error) {
	n.originalWidth = uint(len(n.leaves))

	// make sure that there will not be any left over leaves
	if len(n.leaves)%n.opts.BatchSize != 0 {
		return nil, errors.New("numbers of leaves must be divisible by the batch size")
	}
	// erasure leaves and create the first layer
	err := n.consolidateLeaves()
	if err != nil {
		return nil, err
	}

	// keep consolidating nodes until the root is calculated
	for len(n.layers[len(n.layers)-1]) > 1 {
		nextLayer, err := n.consolidateNodes()
		if err != nil {
			return nil, fmt.Errorf("failure to create new layer: %s", err)
		}
		n.layers = append(n.layers, nextLayer)
	}

	// return the root hash
	hash := n.layers[len(n.layers)-1][0].hash
	return hash, nil
}

// consolidateLeaves extends the leaves in the tree and batches them into single
// nodes as described in the paper
func (n *NCMT) consolidateLeaves() error {
	// erasure the leaf data
	extendedLeaves, err := n.leaves.extend(n.opts.Codec)
	if err != nil {
		return err
	}

	// batchSize is the amount of nodes from each: original and erasured to result in n.opts.BatchSize
	batchSize := n.opts.BatchSize / 2
	// create the next layer
	firstLayer := make(layer, len(n.leaves)/batchSize)

	// batch the original and extended leaves together and combine into a single node
	count := 0
	for i := 0; i < len(n.leaves); i += batchSize {
		j := i + batchSize
		if j > len(n.leaves) {
			j = len(n.leaves)
		}
		// use the first set of original leaves along with their erasures
		batch := append(leaves{}, append(n.leaves[i:j], extendedLeaves[i:j]...)...)
		// to create a new node
		firstLayer[count] = nodeFromLeaves(n.opts.FreshHash(), batch)
		count++
	}

	n.leaves = append(n.leaves, extendedLeaves...)
	n.layers = append(n.layers, firstLayer)

	return nil
}

// consolidateNodes uses the last layer added, along with the erasures of that
// data, to create the next layer of nodes
func (n *NCMT) consolidateNodes() (layer, error) {
	// creates erasure data of the first layer
	latestLayer := n.layers[len(n.layers)-1]
	extendedLayer, err := latestLayer.extend(n.opts.Codec)
	if err != nil {
		return nil, err
	}

	// add to the erasured layer
	n.extendedLayers = append(n.extendedLayers, extendedLayer)

	// batchSize is the initial length of a batch of nodes
	batchSize := n.opts.BatchSize / 2

	// create the next layer
	nextLayer := make(layer, len(latestLayer)/batchSize)

	// batch the original and extended leaves together and combine into a single node
	batchCount := 0
	for i := 0; i < len(latestLayer); i += batchSize {
		j := i + batchSize
		if j > len(latestLayer) {
			j = len(latestLayer)
		}
		batch := append(layer{}, append(latestLayer[i:j], extendedLayer[i:j]...)...)
		nextLayer[batchCount] = newNode(n.opts.FreshHash(), batch)
		batchCount++
	}
	return nextLayer, nil
}
