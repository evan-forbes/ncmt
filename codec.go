package ncmt

import "github.com/lazyledger/rsmt2d"

// Codec wraps methods that erasure data in a NCMT compatable way
type Codec interface {
	Encode([][]byte) ([][]byte, error)
	Decode([][]byte) ([][]byte, error)
	MaxLeaves() int
}

// RSFG8 uses the rsmt2d cached version of the infectious Reed-Solomon forward error
// correction implementation. Not thread safe.
type RSFG8 struct{}

func newRSFG8() RSFG8 {
	return RSFG8{}
}

func (r RSFG8) Encode(input [][]byte) ([][]byte, error) {
	return rsmt2d.Encode(input, rsmt2d.RSGF8)
}

func (r RSFG8) Decode(input [][]byte) ([][]byte, error) {
	return rsmt2d.Decode(input, rsmt2d.RSGF8)
}

func (r RSFG8) MaxLeaves() int {
	return 128
}
