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

/////////////////////////////////////////
// 	Adding Data to the Tree
///////////////////////////////////////

// Push adds data to the leaves of the tree and updates the range. Throws error if data is not pushed
// in order from the lowest (lexographical) id to the greatest
func (n *NCMT) Push(data namespace.Data) error {
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

/////////////////////////////////////////
//  Base Structs: Erasure and Consolidation
///////////////////////////////////////

// layer wraps a slice of nodes
type layer []node

// extend return a new layer of nodes that contain erasured data from the
// original layer
func (l layer) extend(c Codec) (layer, error) {
	extended := make([]node, len(l))

	encodedData, err := c.Encode(l.raw())
	if err != nil {
		return nil, err
	}

	for i, n := range l {
		cleanNode := node{
			min:  n.min,
			max:  n.max,
			hash: encodedData[i],
		}
		extended[i] = cleanNode
	}

	return extended, nil
}

// raw accumlates the hashes of a layer
func (l layer) raw() [][]byte {
	rawData := make([][]byte, len(l))
	for i, node := range l {
		rawData[i] = node.hash
	}
	return rawData
}

type node struct {
	hash     []byte
	parent   *node
	children []node
	min, max namespace.ID
}

// newNode creates a new node using the hashes of the children nodes. Assumes
// children have uniform height (coord.y), len(chilren) != 0, and children nodes
// are presorted by namespace.ID from least to greatest. Uses the format
// min ns(rawData) max ns(rawData) || hash(childHash0 || childHashN...) for the hash
func newNode(h hash.Hash, children []node) node {
	minID := children[0].min
	maxID := children[len(children)-1].max

	// use the position of the first child for
	// gather the hashes of the children nodes
	for _, child := range children {
		h.Write(child.hash)
	}

	return node{
		min:      minID,
		max:      maxID,
		children: children,
		// include the min and max id's in the hash
		hash: h.Sum(append(minID, maxID...)),
	}
}

// nodeFromLeaves creates a new node using the hashes of the children leaves. Assumes
// leaves have uniform height (coord.y), len(chilren) != 0, and children nodes
// are presorted by namespace.ID from least to greatest. uses the format
// min ns(rawData) max ns(rawData) || hash(leafHash0 || leafHashN...) for the hash
func nodeFromLeaves(h hash.Hash, lvs leaves) node {
	minID := lvs[0].min
	maxID := lvs[len(lvs)-1].max

	// use the position of the first child for
	// gather the hashes of the lvs nodes
	for _, child := range lvs {
		h.Write(child.hash)
	}

	return node{
		min:      minID,
		max:      maxID,
		hash:     h.Sum(append(minID, maxID...)),
		children: lvs.nodes(),
	}
}

func (n *node) setParent() {
	for _, child := range n.children {
		child.parent = n
	}
}

type leaves []leaf

// return the nodes of a set of leaves
func (l leaves) nodes() []node {
	out := make([]node, len(l))
	for i, lf := range l {
		out[i] = lf.node
	}
	return out
}

// extend erasures the raw data in the leaves into a new set of leaves that has
// the same namespace.ID prefixed as the original
func (l leaves) extend(c Codec) (leaves, error) {
	extended := make(leaves, len(l))
	encodedLeaves, err := c.Encode(l.raw())
	if err != nil {
		return nil, err
	}

	for i, lf := range l {
		id := make([]byte, lf.data.NamespaceID().Size())
		copy(id, lf.data.NamespaceID())
		newData := namespace.PrefixedDataFrom(id, encodedLeaves[i])
		newLeaf := leaf{
			node: node{
				min: id,
				max: id,
			},
			data: newData,
		}
		// fmt.Println(newLeaf.data.NamespaceID()[0])
		extended[i] = newLeaf
	}

	return extended, nil
}

func (l leaves) raw() [][]byte {
	output := make([][]byte, len(l))
	for i, leaf := range l {
		output[i] = leaf.data.Data()
	}
	return output
}

// leaf is special form of node that holds data
// TODO(evan): just add the data field to node itself to simplify tree
// generation...
type leaf struct {
	node
	data namespace.Data
}

// newLeaf creates a new leaf by hashing the data provided in the format
// ns(rawData) || hash(leafPrefix || rawData)
func newLeaf(h hash.Hash, data namespace.Data) leaf {
	// hash the namespace id along with the
	h.Write(append(data.NamespaceID(), data.Data()...))
	return leaf{
		data: data,
		node: node{
			hash: h.Sum(data.NamespaceID()),
			min:  data.NamespaceID(),
			max:  data.NamespaceID(),
		},
	}
}

// genParityNameSpaceIDs creates filler namespace.ID s for parity data
func genParityNameSpaceID(size int8) namespace.ID {
	var parityID namespace.ID
	for i := int8(0); i < size; i++ {
		parityID = append(parityID, 0xFF)
	}
	return parityID
}
