package ncmt

import (
	"hash"

	"github.com/lazyledger/nmt/namespace"
)

/////////////////////////////////////////
//  Base Structs: Erasuring each layer of a tree
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
		min: minID,
		max: maxID,
		// include the min and max id's in the hash
		hash: h.Sum(append(minID, maxID...)),
	}
}

// nodeFromLeaves creates a new node using the hashes of the children leaves. Assumes
// leaves have uniform height (coord.y), len(chilren) != 0, and children nodes
// are presorted by namespace.ID from least to greatest. uses the format
// min ns(rawData) max ns(rawData) || hash(leafHash0 || leafHashN...) for the hash
func nodeFromLeaves(h hash.Hash, leaves []leaf) node {
	minID := leaves[0].min
	maxID := leaves[len(leaves)-1].max
	// use the position of the first child for
	// gather the hashes of the leaves nodes
	for _, child := range leaves {
		h.Write(child.hash)
	}
	return node{
		min:  minID,
		max:  maxID,
		hash: h.Sum(append(minID, maxID...)),
	}
}

type leaves []leaf

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
		extended[i] = newLeaf
	}
	return extended, nil
}

func (l leaves) raw() [][]byte {
	output := make([][]byte, len(l))
	for i, leaf := range l {
		// var ldata []byte
		// copy(ldata, leaf.data.Data())W
		// output[i] = ldata
		output[i] = leaf.data.Data()
	}
	return output
}

// leaf is special form of node that holds data
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
