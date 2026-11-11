package p2p

// bitfield tracks which pieces a peer has. Piece 0 is the most significant
// bit of the first byte, as described in BEP 3.
type bitfield []byte

func newBitfield(pieceCount int) bitfield {
	return make(bitfield, (pieceCount+7)/8)
}

func (b bitfield) hasPiece(index int) bool {
	byteIndex := index / 8
	if byteIndex < 0 || byteIndex >= len(b) {
		return false
	}
	return b[byteIndex]&(1<<(7-uint(index%8))) != 0
}

func (b bitfield) setPiece(index int) {
	byteIndex := index / 8
	if byteIndex < 0 || byteIndex >= len(b) {
		return
	}
	b[byteIndex] |= 1 << (7 - uint(index%8))
}
