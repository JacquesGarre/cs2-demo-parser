package processing

import "testing"

func TestLoadCalloutResolverUsesMapFilesWithoutDePrefix(t *testing.T) {
	resolver := loadCalloutResolver("de_inferno")
	if resolver == nil {
		t.Fatal("expected inferno callout resolver")
	}

	if got := resolver.LabelForPosition(2000, 292, 208.734); got != "Bombsite A" {
		t.Fatalf("expected Bombsite A for A site coordinates, got %q", got)
	}

	if got := resolver.LabelForPosition(208, 1396, 192.734); got != "Banana" {
		t.Fatalf("expected Banana for banana coordinates, got %q", got)
	}
}

func TestFormatPlayerLabelWithLocation(t *testing.T) {
	if got := formatPlayerLabelWithLocation("ropz", "Palace"); got != "ropz (Palace)" {
		t.Fatalf("expected player label with location, got %q", got)
	}

	if got := formatPlayerLabelWithLocation("ropz", ""); got != "ropz" {
		t.Fatalf("expected plain player label when location missing, got %q", got)
	}
}
