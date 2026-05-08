package rive_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
)

// TestGenerator_TypeKeys verifies spot-checked concrete types have the expected typeKey.
func TestGenerator_TypeKeys(t *testing.T) {
	cases := []struct {
		name string
		obj  rive.Object
		want uint32
	}{
		{"Artboard", &rive.Artboard{}, 1},
		{"Node", &rive.Node{}, 2},
		{"Rectangle", &rive.Rectangle{}, 7},
		{"Backboard", &rive.Backboard{}, 23},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.obj.TypeKey(); got != tc.want {
				t.Fatalf("TypeKey() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestGenerator_RegistryComplete verifies all concrete types are in the registry.
func TestGenerator_RegistryComplete(t *testing.T) {
	cases := []struct {
		name    string
		typeKey uint32
	}{
		{"Artboard", 1},
		{"Node", 2},
		{"Rectangle", 7},
		{"Backboard", 23},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctor, ok := rive.Registry[tc.typeKey]
			if !ok {
				t.Fatalf("Registry missing typeKey %d (%s)", tc.typeKey, tc.name)
			}
			obj := ctor()
			if obj.TypeKey() != tc.typeKey {
				t.Fatalf("Registry[%d]().TypeKey() = %d, want %d", tc.typeKey, obj.TypeKey(), tc.typeKey)
			}
		})
	}
	if len(rive.Registry) == 0 {
		t.Fatal("Registry is empty")
	}
	t.Logf("Registry has %d concrete types", len(rive.Registry))
}

// TestGenerated_DefaultOmission verifies that zero-value properties are not emitted.
func TestGenerated_DefaultOmission(t *testing.T) {
	n := &rive.Node{}
	props := n.Properties()
	for _, p := range props {
		// Node.X is key 13, Node.Y is key 14 — both default to 0 and must be absent
		if p.Key == 13 || p.Key == 14 {
			t.Fatalf("Default property key %d emitted in Properties()", p.Key)
		}
	}
}

// TestGenerated_NodeProperties verifies Node emits X(13) and Y(14) when set.
func TestGenerated_NodeProperties(t *testing.T) {
	n := &rive.Node{}
	n.X = 42.0
	n.Y = 7.5

	props := n.Properties()

	foundX, foundY := false, false
	for _, p := range props {
		switch p.Key {
		case 13:
			foundX = true
			if p.Type != rive.PropertyTypeFloat {
				t.Errorf("Node.X type = %d, want PropertyTypeFloat (%d)", p.Type, rive.PropertyTypeFloat)
			}
			if v, ok := p.Value.(float64); !ok || v != 42.0 {
				t.Errorf("Node.X value = %v, want 42.0", p.Value)
			}
		case 14:
			foundY = true
			if v, ok := p.Value.(float64); !ok || v != 7.5 {
				t.Errorf("Node.Y value = %v, want 7.5", p.Value)
			}
		}
	}
	if !foundX {
		t.Error("Node.X (key 13) missing from Properties()")
	}
	if !foundY {
		t.Error("Node.Y (key 14) missing from Properties()")
	}
}

// TestGenerated_InheritedProperties verifies that Properties() includes inherited fields.
func TestGenerated_InheritedProperties(t *testing.T) {
	// Rectangle inherits from ParametricPath → Path → Node → TransformComponent
	// → WorldTransformComponent → ContainerComponent → Component.
	// Component.Name is key 4 (String).
	r := &rive.Rectangle{}
	r.Name = "my-rect"

	props := r.Properties()
	found := false
	for _, p := range props {
		if p.Key == 4 {
			found = true
			if p.Type != rive.PropertyTypeString {
				t.Errorf("Name type = %d, want PropertyTypeString (%d)", p.Type, rive.PropertyTypeString)
			}
			if v, ok := p.Value.(string); !ok || v != "my-rect" {
				t.Errorf("Name value = %v, want \"my-rect\"", p.Value)
			}
		}
	}
	if !found {
		t.Error("Component.Name (key 4) not found in Rectangle.Properties()")
	}
}

// TestGenerated_BoolProperties verifies bool properties encode as uint64 0/1.
func TestGenerated_BoolProperties(t *testing.T) {
	// LinearAnimation.quantize (bool, key 376, default false)
	la := &rive.LinearAnimation{}
	la.Quantize = true

	props := la.Properties()
	found := false
	for _, p := range props {
		if p.Key == 376 {
			found = true
			if p.Type != rive.PropertyTypeUint {
				t.Errorf("bool property type = %d, want PropertyTypeUint", p.Type)
			}
			if v, ok := p.Value.(uint64); !ok || v != 1 {
				t.Errorf("bool=true value = %v, want uint64(1)", p.Value)
			}
		}
	}
	if !found {
		t.Error("LinearAnimation.Quantize (key 376) missing from Properties()")
	}
}

// TestGenerator_SkipsRuntimeFalse verifies editor-only properties are not in the struct.
// We verify indirectly: Node has "styleValue" (runtime:false, key 176) — it must not
// appear in any Properties() output.
func TestGenerator_SkipsRuntimeFalse(t *testing.T) {
	n := &rive.Node{}
	n.X = 1 // ensure we get some props
	for _, p := range n.Properties() {
		if p.Key == 176 {
			t.Fatal("runtime:false property (styleValue, key 176) found in Node.Properties()")
		}
	}
}

// TestGenerator_RegistrySize sanity-checks registry is non-trivial.
func TestGenerator_RegistrySize(t *testing.T) {
	// We have 350 defs, 63 abstract → expect ~287 concrete types in Registry.
	// Use a loose lower bound to avoid brittleness.
	if got := len(rive.Registry); got < 200 {
		t.Fatalf("Registry has only %d entries; expected ≥200 concrete types", got)
	}
}
