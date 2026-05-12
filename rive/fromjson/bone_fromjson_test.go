package fromjson_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive/fromjson"
)

const boneAnimJSON = `{
  "version": 1,
  "artboard": {
    "name": "Main", "width": 400, "height": 400,
    "bones": [
      {"name": "arm",  "x": 100, "y": 200, "length": 80},
      {"name": "fore", "parent": "arm", "length": 60}
    ],
    "animations": [{
      "name": "wave",
      "duration": 60,
      "tracks": [
        {
          "target": "arm.rotation",
          "keyframes": [
            {"frame": 0,  "value": 0.0},
            {"frame": 30, "value": 3.14},
            {"frame": 60, "value": 6.28}
          ]
        }
      ]
    }]
  }
}`

// TestFromJSON_BoneAnimation verifies that bones in the "bones" array are
// accessible as animation targets via dot-path (e.g. "arm.rotation").
func TestFromJSON_BoneAnimation(t *testing.T) {
	b, err := fromjson.FromJSON([]byte(boneAnimJSON))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Check structural presence: RootBone(41), Bone(40), LinearAnimation(31),
	// KeyedObject(25), KeyedProperty(26), KeyFrameDouble(30)×3.
	counts := map[uint32]int{}
	for _, o := range objects {
		counts[o.TypeKey()]++
	}

	if counts[41] != 1 {
		t.Errorf("want 1 RootBone(41), got %d", counts[41])
	}
	if counts[40] != 1 {
		t.Errorf("want 1 Bone(40), got %d", counts[40])
	}
	if counts[31] != 1 {
		t.Errorf("want 1 LinearAnimation(31), got %d", counts[31])
	}
	if counts[25] != 1 {
		t.Errorf("want 1 KeyedObject(25), got %d", counts[25])
	}
	if counts[26] != 1 {
		t.Errorf("want 1 KeyedProperty(26), got %d", counts[26])
	}
	if counts[30] != 3 {
		t.Errorf("want 3 KeyFrameDouble(30), got %d", counts[30])
	}
}

const boneTranslationJSON = `{
  "version": 1,
  "artboard": {
    "name": "Main", "width": 400, "height": 400,
    "bones": [
      {"name": "spine", "x": 200, "y": 300, "length": 100}
    ],
    "animations": [{
      "name": "slide",
      "tracks": [
        {"target": "spine.translationX", "keyframes": [{"frame": 0, "value": 0}, {"frame": 60, "value": 200}]},
        {"target": "spine.translationY", "keyframes": [{"frame": 0, "value": 0}, {"frame": 60, "value": 50}]}
      ]
    }]
  }
}`

// TestFromJSON_BoneAnimation_TranslationAliases verifies translationX/Y aliases.
func TestFromJSON_BoneAnimation_TranslationAliases(t *testing.T) {
	b, err := fromjson.FromJSON([]byte(boneTranslationJSON))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	counts := map[uint32]int{}
	for _, o := range objects {
		counts[o.TypeKey()]++
	}
	// Two tracks → two KeyedProperty(26), two sets of KeyFrameDouble(30).
	if counts[26] != 2 {
		t.Errorf("want 2 KeyedProperty (one per track), got %d", counts[26])
	}
	if counts[30] != 4 {
		t.Errorf("want 4 KeyFrameDouble (2 per track × 2 tracks), got %d", counts[30])
	}
}
