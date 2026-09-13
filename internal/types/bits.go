package types

import "fmt"

// BitsPositionDiagnostic classifies a BITS position and reports whether it can
// be represented operationally. Large, valid positions retain a style warning.
func BitsPositionDiagnostic(name string, position int64) (code, message string, valid bool) {
	switch {
	case position < 0:
		return DiagBitsNumberNegative, fmt.Sprintf("negative BITS position %d for %q", position, name), false
	case position >= 65535*8:
		return DiagBitsNumberTooLarge, fmt.Sprintf("BITS position %d for %q exceeds maximum (%d)", position, name, 65535*8-1), false
	case position >= 128:
		return DiagBitsNumberLarge, fmt.Sprintf("BITS position %d for %q may cause interoperability problems", position, name), true
	default:
		return "", "", true
	}
}
