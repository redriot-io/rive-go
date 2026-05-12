package fromjson_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// minimalWAVBytes mirrors builder test — 44-byte silent WAV.
var minimalWAVBytes = []byte{
	0x52, 0x49, 0x46, 0x46, 0x24, 0x00, 0x00, 0x00,
	0x57, 0x41, 0x56, 0x45, 0x66, 0x6d, 0x74, 0x20,
	0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
	0x40, 0x1f, 0x00, 0x00, 0x40, 0x1f, 0x00, 0x00,
	0x01, 0x00, 0x08, 0x00, 0x64, 0x61, 0x74, 0x61,
	0x00, 0x00, 0x00, 0x00,
}

const audioJSON = `{
  "version": 1,
  "artboard": {
    "name": "Audio",
    "width": 400,
    "height": 300,
    "audios": [
      {"name": "click", "file": "click.wav"}
    ],
    "children": [
      {"type": "audioEvent", "name": "ClickSound", "audio": "click"}
    ]
  }
}`

// TestFromJSON_Audio_ParseAndVerify verifies AudioAsset + AudioEvent present after build.
func TestFromJSON_Audio_ParseAndVerify(t *testing.T) {
	b, err := fromjson.FromJSONWithAudio([]byte(audioJSON), map[string][]byte{
		"click.wav": minimalWAVBytes,
	})
	if err != nil {
		t.Fatalf("FromJSONWithAudio: %v", err)
	}
	file, err := func() (*rive.File, error) {
		data, err := b.Bytes()
		if err != nil {
			return nil, err
		}
		return rive.ReadBytes(data)
	}()
	if err != nil {
		t.Fatalf("build/read: %v", err)
	}

	// Verify emission order: Backboard(23) → AudioAsset(406) → FileAssetContents(106) → Artboard(1) → AudioEvent(407)
	want := []uint32{23, 406, 106, 1, 407}
	for i, wk := range want {
		if i >= len(file.Objects) {
			t.Fatalf("objects too short at index %d (len=%d)", i, len(file.Objects))
		}
		if file.Objects[i].TypeKey() != wk {
			t.Errorf("objects[%d]: want typeKey %d, got %d", i, wk, file.Objects[i].TypeKey())
		}
	}

	// Verify AudioEvent name = "ClickSound"
	for _, obj := range file.Objects {
		if obj.TypeKey() != 407 {
			continue
		}
		for _, prop := range obj.Properties() {
			if prop.Key == 4 {
				if s, ok := prop.Value.(string); ok && s != "ClickSound" {
					t.Errorf("AudioEvent name: want ClickSound, got %q", s)
				}
			}
		}
	}
}

// TestFromJSON_Audio_MissingRef verifies a clear error when audio name is not defined.
func TestFromJSON_Audio_MissingRef(t *testing.T) {
	badJSON := `{
  "version": 1,
  "artboard": {
    "name": "Audio",
    "width": 400,
    "height": 300,
    "children": [
      {"type": "audioEvent", "name": "boom", "audio": "nonexistent"}
    ]
  }
}`
	_, err := fromjson.FromJSONWithAudio([]byte(badJSON), nil)
	if err == nil {
		t.Fatal("expected error for missing audio ref, got nil")
	}
	if msg := err.Error(); msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

// TestFromJSON_Audio_RoundTrip verifies build → WriteBytes → ReadBytes preserves structure.
func TestFromJSON_Audio_RoundTrip(t *testing.T) {
	b, err := fromjson.FromJSONWithAudio([]byte(audioJSON), map[string][]byte{
		"click.wav": minimalWAVBytes,
	})
	if err != nil {
		t.Fatalf("FromJSONWithAudio: %v", err)
	}
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	file, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	typeKeySet := map[uint32]bool{}
	for _, obj := range file.Objects {
		typeKeySet[obj.TypeKey()] = true
	}
	for _, tk := range []uint32{23, 406, 106, 1, 407} {
		if !typeKeySet[tk] {
			t.Errorf("missing typeKey %d in round-tripped file", tk)
		}
	}
}
