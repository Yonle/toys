package main

func moveToFront(s []byte, start int) {
	// Get the segment to move
	segment := s[start:]

	// Calculate how many elements need to be shifted
	n := len(segment)

	// Use copy to shift elements
	copy(s, segment)

	// Fill the remaining space with original elements
	copy(s[n:], s[:start])
}
