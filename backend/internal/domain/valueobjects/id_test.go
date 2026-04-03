package valueobjects

import "testing"

func TestNewID_HexLengthAndNotFallback(t *testing.T) {
	id := NewID()

	if id == "fallback-id" {
		t.Fatalf("expected generated ID, got fallback")
	}

	if len(id) != 24 {
		t.Fatalf("expected 24-char hex id, got len=%d value=%q", len(id), id)
	}

	for _, ch := range id {
		isDigit := ch >= '0' && ch <= '9'
		isLowerHex := ch >= 'a' && ch <= 'f'
		if !isDigit && !isLowerHex {
			t.Fatalf("expected hex string, got invalid char %q in %q", ch, id)
		}
	}
}

func TestNewID_GeneratesDifferentValues(t *testing.T) {
	first := NewID()
	second := NewID()

	if first == second {
		t.Fatalf("expected different IDs, got identical value %q", first)
	}
}
