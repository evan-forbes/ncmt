package ncmt

// Proof describes the data needed to verify inclusion of some data in a NCMT
type Proof struct {
	Set    [][]byte
	Root   []byte
	Index  uint
	Leaves uint
}

// return a simpler more direct proof and serialize later
// just use a RS decoder to 'peel' back layers

// func Verify(h hash.Hash, p Proof) bool {

// }

// func (n *NCMT) ProveNamespace(nID namespace.ID) (Proof, error) {
// 	// find the namespace or return an error
// 	found, start, end := n.foundInRange(nID)
// 	if !found {
// 		return Proof{}, fmt.Errorf("names not found in tree: %s", string(nID))
// 	}
// 	// build proof
// 	return Proof{}, nil
// }

func (n *NCMT) ProveRange(start, end uint) (Proof, error) {
	// check that the range is valid
	if end < uint(len(n.leaves)) && start <= end {

	}
	return Proof{}, nil
}

// // planProofRange determines the nodes that are needed to prove inclusion of a
// // given range
// func (n *NCMT) planProofRange(start, end uint) {
// 	//
// 	return nil
// }

// TODO: keep erasured leaves separate

// func (n *NCMT) ProveLeaf(idx uint) (Proof, error) {
// 	// check range
// 	if idx > uint(len(n.leaves)) {
// 		return Proof{}, fmt.Errorf(
// 			"leaf out of range: max range %d, id given %d",
// 			len(n.leaves),
// 			idx,
// 		)
// 	}

// 	// iterate through each layer
// 	heritage := make([]node, len(n.layers))
// 	nextIndx := idx
// 	for i, l := range n.layers {
// 		heritage[i] = l[nextIndx]
// 		nextIndx = nextIndx / 2
// 	}
// 	// expand each node into the hashes
// }
