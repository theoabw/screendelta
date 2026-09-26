package identity

// Signature is a coarse summary of how an element looks, as distinct from where it is.
//
// It exists because geometry cannot answer the question the identity layer is asked. A changed area
// that contains a tracked element is consistent with a cover, a replacement, a growth that repainted
// itself and a full repaint, and every one of those is consistent with the same rectangle. Appearance
// is the one extra piece of evidence that separates them, and this is deliberately the cheapest
// version of it that works: sixteen bytes of luma means over an element's footprint.
//
// It is evidence and never proof, which is why it decides an identity's uncertainty rather than
// asserting a match. Two elements that look identical cannot be told apart by it, and the failure
// mode is a reacquisition marked uncertain rather than a confident wrong answer.
const (
	// SignatureCells is the number of cells in a signature, a four by four grid.
	SignatureCells = 16
	// SignatureGrid is the edge of that grid.
	SignatureGrid = 4
	// SignatureTolerance is how far two signatures may differ per cell and still describe the same
	// thing. It is an average rather than a total, for the same reason the noise floor is per cell:
	// a total would let one cell differ by the whole allowance while fifteen agree, and capture noise
	// is a per cell phenomenon.
	//
	// Two frames of the same static screen differ by capture noise, which the noise floor bounds at
	// around six levels per cell; a repaint of an element's contents moves its cell means much
	// further. The value sits between the two.
	SignatureTolerance = 12
)

// Signature holds one mean per cell, row by row, quantised to the byte range the luma plane uses.
type Signature [SignatureCells]uint8

// Distance is the sum of the absolute differences between two signatures, which is the measure the
// tolerance is stated in.
func (s Signature) Distance(other Signature) int {
	total := 0
	for index := range s {
		difference := int(s[index]) - int(other[index])
		if difference < 0 {
			difference = -difference
		}
		total += difference
	}
	return total
}

// Close reports whether two signatures are near enough to describe the same thing, on average per
// cell, so the allowance means the same thing whatever the grid resolution is set to.
func (s Signature) Close(other Signature) bool {
	return s.Distance(other) <= SignatureTolerance*SignatureCells
}

// ClosestTo returns the index of the signature nearest to the reader's, and the distance to it.
//
// The comparison that matters is relative rather than absolute: a returning element is recognised
// because it looks more like the one that left than like anything currently on the screen, which is
// why the caller compares distances rather than testing one against the tolerance.
func (s Signature) ClosestTo(others []Signature) (int, int) {
	best, bestDistance := -1, 0
	for index, other := range others {
		distance := s.Distance(other)
		if best < 0 || distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best, bestDistance
}
