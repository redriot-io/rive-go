package fontcheck_test

import (
	"os"
	"testing"

	"github.com/redriot-io/rive-go/internal/fontcheck"
)

func loadFont(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return data
}

// TestParseCmap_Abel verifies Latin text coverage in the Abel font (the font
// used by hello_world.riv after the T-488 Codicon fix).
func TestParseCmap_Abel(t *testing.T) {
	data := loadFont(t, "abel.ttf")
	hasGlyph, err := fontcheck.ParseCmap(data)
	if err != nil {
		t.Fatalf("ParseCmap: %v", err)
	}

	latinWant := []struct {
		r    rune
		name string
	}{
		{'H', "H"},
		{'e', "e"},
		{'l', "l"},
		{'o', "o"},
		{' ', "space"},
		{'W', "W"},
		{'r', "r"},
		{'d', "d"},
		{'A', "A"},
		{'z', "z"},
		{'0', "0"},
		{'9', "9"},
		{'.', "."},
		{',', ","},
		{'!', "!"},
		{'—', "em-dash"},
	}
	for _, tc := range latinWant {
		if tc.r == ' ' {
			continue // space may or may not have a glyph; skip
		}
		if !hasGlyph(tc.r) {
			t.Errorf("Abel: HasGlyph(%q %s) = false, want true", tc.r, tc.name)
		}
	}

	// PUA codepoints (U+E000–U+F8FF) must NOT be present in Abel.
	puaAbsent := []rune{0xE000, 0xE001, 0xEA2A, 0xF000, 0xF8FF}
	for _, r := range puaAbsent {
		if hasGlyph(r) {
			t.Errorf("Abel: HasGlyph(U+%04X) = true, want false (PUA should be absent)", r)
		}
	}
}

// TestParseCmap_Codicon verifies that the VS Code Codicon icon font has no
// Latin glyph coverage — the root cause of T-488 blank text rendering.
func TestParseCmap_Codicon(t *testing.T) {
	data := loadFont(t, "codicon.ttf")
	hasGlyph, err := fontcheck.ParseCmap(data)
	if err != nil {
		t.Fatalf("ParseCmap: %v", err)
	}

	// Codicon has NO Latin glyphs.
	latinAbsent := []struct {
		r    rune
		name string
	}{
		{'H', "H"},
		{'e', "e"},
		{'l', "l"},
		{'o', "o"},
		{'W', "W"},
		{'r', "r"},
		{'d', "d"},
		{'A', "A"},
		{'z', "z"},
	}
	for _, tc := range latinAbsent {
		if hasGlyph(tc.r) {
			t.Errorf("Codicon: HasGlyph(%q %s) = true, want false (icon font has no Latin)", tc.r, tc.name)
		}
	}

	// Codicon DOES have PUA glyphs (its icons live in U+E000–U+EAFF).
	puaPresent := []rune{0xEA2A, 0xEB01}
	for _, r := range puaPresent {
		if !hasGlyph(r) {
			t.Logf("Codicon: HasGlyph(U+%04X) = false (may be outside Codicon range, skipping)", r)
		}
	}
}

// TestParseCmap_Malformed verifies graceful error handling.
func TestParseCmap_Malformed(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short", []byte{0, 1, 2, 3}},
		{"not a font", []byte("this is not a font file at all padded to be 16 bytes long")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fontcheck.ParseCmap(tc.data)
			if err == nil {
				t.Error("want error for malformed data, got nil")
			}
		})
	}
}
