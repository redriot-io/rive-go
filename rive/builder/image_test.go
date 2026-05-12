package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// minimalPNG is a 1×1 transparent PNG (67 bytes).
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR length + type
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1×1
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // bit depth=8, colorType=2
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT length + type
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, // IEND
	0x44, 0xae, 0x42, 0x60, 0x82,
}

func buildImageScene(t *testing.T) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	asset := ab.EmbedImage("myImage", minimalPNG)
	ab.Image(asset).Position(250, 250)
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objects
}

// TestImage_EmissionOrder verifies: Backboard → ImageAsset → FileAssetContents → Artboard → Image
func TestImage_EmissionOrder(t *testing.T) {
	objects := buildImageScene(t)
	if len(objects) < 5 {
		t.Fatalf("expected ≥5 objects, got %d", len(objects))
	}
	if objects[0].TypeKey() != 23 {
		t.Errorf("[0] want Backboard(23), got %d", objects[0].TypeKey())
	}
	if objects[1].TypeKey() != 105 {
		t.Errorf("[1] want ImageAsset(105), got %d", objects[1].TypeKey())
	}
	if objects[2].TypeKey() != 106 {
		t.Errorf("[2] want FileAssetContents(106), got %d", objects[2].TypeKey())
	}
	if objects[3].TypeKey() != 1 {
		t.Errorf("[3] want Artboard(1), got %d", objects[3].TypeKey())
	}
	if objects[4].TypeKey() != 100 {
		t.Errorf("[4] want Image(100), got %d", objects[4].TypeKey())
	}
}

// TestImage_AssetIdIndex verifies Image.assetId=0 (first ImageAsset, 0-indexed).
func TestImage_AssetIdIndex(t *testing.T) {
	objects := buildImageScene(t)
	var assetId uint64
	found := false
	for _, obj := range objects {
		if obj.TypeKey() != 100 {
			continue
		}
		for _, prop := range obj.Properties() {
			if prop.Key == 206 {
				assetId = prop.Value.(uint64)
				found = true
			}
		}
	}
	if !found {
		t.Fatal("Image node not found or assetId property missing")
	}
	if assetId != 0 {
		t.Errorf("assetId: want 0, got %d", assetId)
	}
}

// TestImage_FileAssetContentsBytes verifies embedded PNG bytes land on FileAssetContents.
func TestImage_FileAssetContentsBytes(t *testing.T) {
	objects := buildImageScene(t)
	var gotBytes []byte
	for _, obj := range objects {
		if obj.TypeKey() != 106 {
			continue
		}
		for _, prop := range obj.Properties() {
			if prop.Key == 212 {
				gotBytes = prop.Value.([]byte)
			}
		}
	}
	if len(gotBytes) == 0 {
		t.Fatal("FileAssetContents bytes not found")
	}
	if len(gotBytes) != len(minimalPNG) {
		t.Errorf("bytes length: want %d, got %d", len(minimalPNG), len(gotBytes))
	}
}

// TestImage_RoundTrip verifies build → WriteBytes → ReadBytes preserves all objects.
func TestImage_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	asset := ab.EmbedImage("myImage", minimalPNG)
	ab.Image(asset).Position(100, 200)
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	file, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	typeKeys := make([]uint32, len(file.Objects))
	for i, o := range file.Objects {
		typeKeys[i] = o.TypeKey()
	}
	// Must contain typeKeys 23, 105, 106, 1, 100 in order
	want := []uint32{23, 105, 106, 1, 100}
	for i, wk := range want {
		if i >= len(typeKeys) {
			t.Fatalf("objects too short at index %d", i)
		}
		if typeKeys[i] != wk {
			t.Errorf("objects[%d]: want typeKey %d, got %d", i, wk, typeKeys[i])
		}
	}
}
