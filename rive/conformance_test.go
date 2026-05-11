package rive_test

import (
	"os"
	"testing"

	"github.com/redriot-io/rive-go/rive"
)

// TestConformance_OfficialToC inspects official Rive editor exports to determine
// how bytes-typed properties (key 212, 223) appear in the ToC.
// This is the ground-truth test for our writer's ToC behavior.
//
// Confirmed ground truth (run 2026-05-11):
//   hello_world.riv:  key 212 IN ToC, field-index=1 (PropertyTypeString proxy)
//   new_text.riv:     key 212 IN ToC, field-index=1
//   ellipsis.riv:     key 212 IN ToC, field-index=1
//   ball_test.riv:    key 212 NOT in ToC (no embedded font)
//   blend_test.riv:   key 212 NOT in ToC (no embedded font)
func TestConformance_OfficialToC(t *testing.T) {
	files := []struct {
		path string
		desc string
	}{
		{"testdata/official/hello_world.riv", "1 Text, embedded font (official)"},
		{"testdata/official/new_text.riv", "multi-style text, embedded font (official)"},
		{"testdata/official/ellipsis.riv", "text overflow, embedded font (official)"},
		{"testdata/official/ball_test.riv", "shapes + state machine, no font (official)"},
		{"testdata/official/blend_test.riv", "blend state machine, no font (official)"},
	}

	bytesKeys := []struct {
		key  uint32
		name string
	}{
		{212, "FileAssetContents.bytes"},
		{223, "Mesh.triangleIndexBytes"},
		{588, "DataBind.sourcePathIds"},
		{920, "DataBind.path"},
		{963, "ViewModelInstanceListItem.viewModelPathIds"},
	}

	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Logf("SKIP %s — not available", f.path)
			continue
		}

		parsed, err := rive.ReadBytes(data)
		if err != nil {
			t.Errorf("FAIL parse %s: %v", f.path, err)
			continue
		}

		t.Logf("=== %s (%s, %d bytes, major=%d) ===",
			f.path, f.desc, len(data), parsed.MajorVersion)

		anyFound := false
		for _, bk := range bytesKeys {
			pt, inToC := parsed.PropertyTypeOf(bk.key)
			if inToC {
				t.Logf("  key %3d (%s): IN ToC  field-index=%d", bk.key, bk.name, pt)
				anyFound = true
			} else {
				t.Logf("  key %3d (%s): NOT in ToC", bk.key, bk.name)
			}
		}
		if !anyFound {
			t.Logf("  (no bytes-type keys found in ToC — omit approach confirmed)")
		}
	}
}

// TestConformance_OurWriterMatchesOfficial ensures our writer's ToC output
// matches the official encoder's behavior for bytes properties (key 212).
//
// Ground truth (from TestConformance_OfficialToC output):
//   - official hello_world.riv: key 212 IN ToC, field-index=1 (PropertyTypeString proxy)
//   - official new_text.riv:    key 212 IN ToC, field-index=1
//   - official ellipsis.riv:    key 212 IN ToC, field-index=1
func TestConformance_OurWriterMatchesOfficial(t *testing.T) {
	officialData, err := os.ReadFile("testdata/official/hello_world.riv")
	if err != nil {
		t.Skip("testdata/official/hello_world.riv not available")
	}
	officialFile, err := rive.ReadBytes(officialData)
	if err != nil {
		t.Fatalf("parse official hello_world.riv: %v", err)
	}

	oursData, err := os.ReadFile("../docs/preview/fromjson_hello_world.riv")
	if err != nil {
		t.Skip("docs/preview/fromjson_hello_world.riv not available")
	}
	oursFile, err := rive.ReadBytes(oursData)
	if err != nil {
		t.Fatalf("parse our fromjson_hello_world.riv: %v", err)
	}

	officialType, officialHas212 := officialFile.PropertyTypeOf(212)
	oursType, oursHas212 := oursFile.PropertyTypeOf(212)

	t.Logf("official: key 212 in ToC=%v field-index=%d", officialHas212, officialType)
	t.Logf("ours:     key 212 in ToC=%v field-index=%d", oursHas212, oursType)

	if !oursHas212 {
		t.Error("our writer omitted key 212 from ToC — must include with field-index=1 matching official encoder")
	}
	if officialHas212 != oursHas212 {
		t.Errorf("ToC presence mismatch for key 212: official=%v, ours=%v", officialHas212, oursHas212)
	}
	if officialHas212 && oursHas212 && officialType != oursType {
		t.Errorf("ToC field-index mismatch for key 212: official=%d, ours=%d", officialType, oursType)
	}
}

// TestConformance_OfficialFilesReadable ensures all downloaded official files
// parse without error. A parse failure here means our reader is broken.
func TestConformance_OfficialFilesReadable(t *testing.T) {
	files := []string{
		"testdata/official/hello_world.riv",
		"testdata/official/new_text.riv",
		"testdata/official/ellipsis.riv",
		"testdata/official/ball_test.riv",
		"testdata/official/blend_test.riv",
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("not available: %v", err)
			}

			f, err := rive.ReadBytes(data)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(f.Objects) == 0 {
				t.Error("no objects parsed")
			}
			t.Logf("ok: %d objects, major=%d", len(f.Objects), f.MajorVersion)
		})
	}
}
