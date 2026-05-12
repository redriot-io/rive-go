package fromjson_test

import (
	"strings"
	"testing"

	"github.com/redriot-io/rive-go/rive/fromjson"
)

// minimalPNGBytes is a 1×1 transparent PNG used for injection-based tests.
var minimalPNGBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

const imageScene = `{
  "version": 1,
  "artboard": {
    "name": "Main",
    "width": 500,
    "height": 500,
    "images": [{"name": "bg", "file": "bg.png"}],
    "children": [{
      "type": "image",
      "name": "background",
      "x": 250, "y": 250,
      "image": "bg"
    }]
  }
}`

// TestFromJSON_Image_ParseAndEmit verifies that an image JSON scene produces
// an ImageAsset (typeKey 105) and Image node (typeKey 100) in the output.
func TestFromJSON_Image_ParseAndEmit(t *testing.T) {
	b, err := fromjson.FromJSONWithImages([]byte(imageScene), map[string][]byte{
		"bg.png": minimalPNGBytes,
	})
	if err != nil {
		t.Fatalf("FromJSONWithImages: %v", err)
	}
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var hasImageAsset, hasImageNode bool
	for _, o := range objects {
		switch o.TypeKey() {
		case 105: // ImageAsset
			hasImageAsset = true
		case 100: // Image drawable node
			hasImageNode = true
		}
	}
	if !hasImageAsset {
		t.Error("expected ImageAsset (typeKey 105) in output objects")
	}
	if !hasImageNode {
		t.Error("expected Image node (typeKey 100) in output objects")
	}
}

// TestFromJSON_Image_MissingRef verifies that referencing an undefined image name
// returns a descriptive error.
func TestFromJSON_Image_MissingRef(t *testing.T) {
	const badScene = `{
  "version": 1,
  "artboard": {
    "name": "Main",
    "width": 500,
    "height": 500,
    "images": [{"name": "bg", "file": "bg.png"}],
    "children": [{
      "type": "image",
      "name": "hero",
      "x": 100, "y": 100,
      "image": "nonexistent"
    }]
  }
}`
	_, err := fromjson.FromJSONWithImages([]byte(badScene), map[string][]byte{
		"bg.png": minimalPNGBytes,
	})
	if err == nil {
		t.Fatal("expected error for undefined image ref, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the missing image name; got: %v", err)
	}
}
