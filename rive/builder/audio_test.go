package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// minimalWAV is a tiny 44-byte valid WAV file (silent, 8-bit mono 8kHz, 0 samples).
var minimalWAV = []byte{
	0x52, 0x49, 0x46, 0x46, // "RIFF"
	0x24, 0x00, 0x00, 0x00, // chunk size = 36
	0x57, 0x41, 0x56, 0x45, // "WAVE"
	0x66, 0x6d, 0x74, 0x20, // "fmt "
	0x10, 0x00, 0x00, 0x00, // subchunk1 size = 16
	0x01, 0x00,             // PCM
	0x01, 0x00,             // mono
	0x40, 0x1f, 0x00, 0x00, // 8000 Hz
	0x40, 0x1f, 0x00, 0x00, // byte rate
	0x01, 0x00,             // block align
	0x08, 0x00,             // 8-bit
	0x64, 0x61, 0x74, 0x61, // "data"
	0x00, 0x00, 0x00, 0x00, // 0 samples
}

func buildAudioScene(t *testing.T) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	asset := ab.EmbedAudio("footstep", minimalWAV)
	ab.AudioEvent("Footstep", asset)
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objects
}

// TestAudio_EmissionOrder verifies: Backboard → AudioAsset → FileAssetContents → Artboard → AudioEvent
func TestAudio_EmissionOrder(t *testing.T) {
	objects := buildAudioScene(t)
	if len(objects) < 5 {
		t.Fatalf("expected ≥5 objects, got %d", len(objects))
	}
	want := []uint32{23, 406, 106, 1, 407}
	for i, wk := range want {
		if objects[i].TypeKey() != wk {
			t.Errorf("objects[%d]: want typeKey %d, got %d", i, wk, objects[i].TypeKey())
		}
	}
}

// TestAudio_AssetIdIndex verifies AudioEvent.assetId=0 (first AudioAsset, 0-indexed).
func TestAudio_AssetIdIndex(t *testing.T) {
	objects := buildAudioScene(t)
	for _, obj := range objects {
		if obj.TypeKey() != 407 {
			continue
		}
		for _, prop := range obj.Properties() {
			if prop.Key == 408 {
				if v := prop.Value.(uint64); v != 0 {
					t.Errorf("AudioEvent assetId: want 0, got %d", v)
				}
				return
			}
		}
		t.Fatal("AudioEvent has no assetId property (key 408)")
	}
	t.Fatal("no AudioEvent (typeKey=407) found")
}

// TestAudio_FileAssetContentsBytes verifies embedded WAV bytes land on FileAssetContents.
func TestAudio_FileAssetContentsBytes(t *testing.T) {
	objects := buildAudioScene(t)
	for _, obj := range objects {
		if obj.TypeKey() != 106 {
			continue
		}
		for _, prop := range obj.Properties() {
			if prop.Key == 212 {
				b := prop.Value.([]byte)
				if len(b) != len(minimalWAV) {
					t.Errorf("bytes length: want %d, got %d", len(minimalWAV), len(b))
				}
				return
			}
		}
	}
	t.Fatal("FileAssetContents bytes not found")
}

// TestAudio_RoundTrip verifies build → WriteBytes → ReadBytes preserves all objects.
func TestAudio_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	asset := ab.EmbedAudio("footstep", minimalWAV, builder.WithVolume(0.8))
	ab.AudioEvent("Footstep", asset)
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	file, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	want := []uint32{23, 406, 106, 1, 407}
	for i, wk := range want {
		if i >= len(file.Objects) {
			t.Fatalf("objects too short at index %d", i)
		}
		if file.Objects[i].TypeKey() != wk {
			t.Errorf("objects[%d]: want typeKey %d, got %d", i, wk, file.Objects[i].TypeKey())
		}
	}
}
