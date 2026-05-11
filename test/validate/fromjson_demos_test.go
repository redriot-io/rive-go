// Phase 4D integration tests: FromJSON SM demo pipeline.
// Compiles each JSON demo file and verifies the resulting .riv structure.
// Runs with go test ./... (no build tags).
package validate_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// repoRoot returns the absolute path to the repo root by walking up from this file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is test/validate/fromjson_demos_test.go → go up two dirs
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func compileDemo(t *testing.T, jsonPath string) *rive.File {
	t.Helper()
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", jsonPath, err)
	}
	b, err := fromjson.FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON %s: %v", jsonPath, err)
	}
	raw, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes %s: %v", jsonPath, err)
	}
	f, err := rive.ReadBytes(raw)
	if err != nil {
		t.Fatalf("ReadBytes %s: %v", jsonPath, err)
	}
	return f
}

func countTypeKey(f *rive.File, tk uint32) int {
	n := 0
	for _, o := range f.Objects {
		if o.TypeKey() == tk {
			n++
		}
	}
	return n
}

// ── Toggle Button SM ─────────────────────────────────────────────────────────

func TestFromJSONDemo_ToggleButtonSM(t *testing.T) {
	root := repoRoot(t)
	f := compileDemo(t, filepath.Join(root, "docs/preview/fromjson/toggle_button_sm.json"))

	// 1 StateMachine
	if n := countTypeKey(f, 53); n != 1 {
		t.Errorf("StateMachine count: got %d, want 1", n)
	}
	// 1 StateMachineBool ("active")
	if n := countTypeKey(f, 59); n != 1 {
		t.Errorf("StateMachineBool count: got %d, want 1", n)
	}
	// 2 AnimationStates (Off, On)
	if n := countTypeKey(f, 61); n != 2 {
		t.Errorf("AnimationState count: got %d, want 2", n)
	}
	// 2 StateMachineListenerSingle (pointer_down, pointer_up)
	if n := countTypeKey(f, 114); n != 2 {
		t.Errorf("StateMachineListenerSingle count: got %d, want 2", n)
	}
	// 2 ListenerBoolChange (one per listener)
	if n := countTypeKey(f, 117); n != 2 {
		t.Errorf("ListenerBoolChange count: got %d, want 2", n)
	}
	// 2 LinearAnimations (idle_off, idle_on)
	if n := countTypeKey(f, 31); n != 2 {
		t.Errorf("LinearAnimation count: got %d, want 2", n)
	}
}

// ── Multi-State Nav SM ───────────────────────────────────────────────────────

func TestFromJSONDemo_MultistateNavSM(t *testing.T) {
	root := repoRoot(t)
	f := compileDemo(t, filepath.Join(root, "docs/preview/fromjson/multistate_nav_sm.json"))

	// 1 StateMachine
	if n := countTypeKey(f, 53); n != 1 {
		t.Errorf("StateMachine count: got %d, want 1", n)
	}
	// 1 StateMachineNumber ("page")
	if n := countTypeKey(f, 56); n != 1 {
		t.Errorf("StateMachineNumber count: got %d, want 1", n)
	}
	// 3 AnimationStates (A, B, C)
	if n := countTypeKey(f, 61); n != 3 {
		t.Errorf("AnimationState count: got %d, want 3", n)
	}
	// 5 StateTransitions: 4 explicit (A↔B, B↔C) + 1 implicit entry-state edge
	if n := countTypeKey(f, 65); n != 5 {
		t.Errorf("StateTransition count: got %d, want 5", n)
	}
	// 4 TransitionNumberConditions (one per transition)
	if n := countTypeKey(f, 70); n != 4 {
		t.Errorf("TransitionNumberCondition count: got %d, want 4", n)
	}
	// 3 LinearAnimations (page_a, page_b, page_c)
	if n := countTypeKey(f, 31); n != 3 {
		t.Errorf("LinearAnimation count: got %d, want 3", n)
	}
}

// ── Blend Slider SM ──────────────────────────────────────────────────────────

func TestFromJSONDemo_BlendSliderSM(t *testing.T) {
	root := repoRoot(t)
	f := compileDemo(t, filepath.Join(root, "docs/preview/fromjson/blend_slider_sm.json"))

	// 1 StateMachine
	if n := countTypeKey(f, 53); n != 1 {
		t.Errorf("StateMachine count: got %d, want 1", n)
	}
	// 1 StateMachineNumber ("mix")
	if n := countTypeKey(f, 56); n != 1 {
		t.Errorf("StateMachineNumber count: got %d, want 1", n)
	}
	// 1 BlendState1DInput (the Blend state)
	if n := countTypeKey(f, 76); n != 1 {
		t.Errorf("BlendState1DInput count: got %d, want 1", n)
	}
	// 2 BlendAnimation1D (gentle + intense entries)
	if n := countTypeKey(f, 75); n != 2 {
		t.Errorf("BlendAnimation1D count: got %d, want 2", n)
	}
	// 1 StateTransition (ExitState → Blend)
	if n := countTypeKey(f, 65); n != 1 {
		t.Errorf("StateTransition count: got %d, want 1", n)
	}
	// 2 LinearAnimations (gentle, intense)
	if n := countTypeKey(f, 31); n != 2 {
		t.Errorf("LinearAnimation count: got %d, want 2", n)
	}
}

// ── rivtool verify pipeline ──────────────────────────────────────────────────
// Verify that the pre-compiled .riv files (checked into docs/preview/) pass
// the same structural checks that `rivtool verify` performs.

func runVerify(t *testing.T, rivPath string) {
	t.Helper()
	data, err := os.ReadFile(rivPath)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", rivPath, err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes %s: %v", rivPath, err)
	}
	// Basic structural checks matching cmdVerify logic:
	// artboard present, parentId valid (ReadBytes would error on most issues).
	if len(f.Objects) == 0 {
		t.Errorf("%s: no objects", rivPath)
	}
	// At least one Artboard
	artboards := countTypeKey(f, 1)
	if artboards == 0 {
		t.Errorf("%s: no Artboard", rivPath)
	}
	// At least one StateMachine
	sms := countTypeKey(f, 53)
	if sms == 0 {
		t.Errorf("%s: no StateMachine", rivPath)
	}
}

func TestRivtoolVerify_FromJSONDemos(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		"docs/preview/fromjson_toggle_button_sm.riv",
		"docs/preview/fromjson_multistate_nav_sm.riv",
		"docs/preview/fromjson_blend_slider_sm.riv",
	}
	for _, rel := range files {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			runVerify(t, filepath.Join(root, rel))
		})
	}
}
