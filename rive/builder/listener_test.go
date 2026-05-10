package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

const tkStateMachineListenerSingle = 114

func TestListener_SinglePointerDown(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000).Name("button")

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerDown)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 1 {
		t.Fatalf("expected 1 StateMachineListenerSingle, got %d", len(listeners))
	}

	obj := listeners[0].(*rive.StateMachineListenerSingle)
	// PointerDown = 0, which is suppressed (default) — check TargetId instead
	if obj.TargetId == ^uint64(0) {
		t.Error("TargetId should be set, got sentinel")
	}
	// ListenerTypeValue=0 is the default (PointerDown), not emitted as property but field should be 0
	if obj.ListenerTypeValue != 0 {
		t.Errorf("ListenerTypeValue = %d, want 0 (PointerDown)", obj.ListenerTypeValue)
	}
}

func TestListener_PointerEnter(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFF00FF00).Name("hotspot")

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerEnter)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 1 {
		t.Fatalf("expected 1 StateMachineListenerSingle, got %d", len(listeners))
	}

	obj := listeners[0].(*rive.StateMachineListenerSingle)
	if obj.ListenerTypeValue != uint64(builder.ListenerPointerEnter) {
		t.Errorf("ListenerTypeValue = %d, want %d (PointerEnter)", obj.ListenerTypeValue, builder.ListenerPointerEnter)
	}
}

func TestListener_Click(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(200, 200, 80, 80).Fill(0xFF0000FF).Name("btn")

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerClick)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 1 {
		t.Fatalf("expected 1 StateMachineListenerSingle, got %d", len(listeners))
	}

	obj := listeners[0].(*rive.StateMachineListenerSingle)
	if obj.ListenerTypeValue != uint64(builder.ListenerClick) {
		t.Errorf("ListenerTypeValue = %d, want %d (Click)", obj.ListenerTypeValue, builder.ListenerClick)
	}
}

func TestListener_MultipleListeners(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	s1 := ab.Rectangle(50, 50, 40, 40).Fill(0xFFFF0000).Name("s1")
	s2 := ab.Rectangle(150, 50, 40, 40).Fill(0xFF00FF00).Name("s2")

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(s1, builder.ListenerPointerDown)
	sm.Listener(s2, builder.ListenerPointerExit)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 2 {
		t.Fatalf("expected 2 StateMachineListenerSingle, got %d", len(listeners))
	}

	l1 := listeners[0].(*rive.StateMachineListenerSingle)
	l2 := listeners[1].(*rive.StateMachineListenerSingle)

	if l1.ListenerTypeValue != uint64(builder.ListenerPointerDown) {
		t.Errorf("l1 ListenerTypeValue = %d, want 0 (PointerDown)", l1.ListenerTypeValue)
	}
	if l2.ListenerTypeValue != uint64(builder.ListenerPointerExit) {
		t.Errorf("l2 ListenerTypeValue = %d, want %d (PointerExit)", l2.ListenerTypeValue, builder.ListenerPointerExit)
	}

	// TargetIds must differ (different shapes)
	if l1.TargetId == l2.TargetId {
		t.Errorf("l1.TargetId (%d) == l2.TargetId (%d), want distinct", l1.TargetId, l2.TargetId)
	}
}

func TestListener_TargetIdIsArtboardRelative(t *testing.T) {
	// TargetId must equal the artboard-relative index of the target shape.
	// artboard-relative index = global_stream_index - artboard_global_index.
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Ellipse(200, 200, 60, 60).Fill(0xFFFFFFFF).Name("target")

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerUp)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Find artboard global index (typeKey=1) and Shape global index (typeKey=3).
	artboardGlobal := -1
	shapeGlobal := -1
	for i, o := range objects {
		switch o.TypeKey() {
		case 1: // Artboard
			artboardGlobal = i
		case 3: // Shape container
			shapeGlobal = i
		}
	}
	if artboardGlobal < 0 {
		t.Fatal("artboard not found in stream")
	}
	if shapeGlobal < 0 {
		t.Fatal("shape not found in stream")
	}

	expectedTargetId := uint64(shapeGlobal - artboardGlobal)

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(listeners))
	}
	obj := listeners[0].(*rive.StateMachineListenerSingle)
	if obj.TargetId != expectedTargetId {
		t.Errorf("TargetId = %d, want %d (artboard-relative)", obj.TargetId, expectedTargetId)
	}
}

func TestListener_TypeKeyIs114(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000)

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerMove)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found := false
	for _, o := range objects {
		if o.TypeKey() == tkStateMachineListenerSingle {
			found = true
			break
		}
	}
	if !found {
		t.Error("no object with typeKey=114 found in stream")
	}
}

func TestListener_EmittedAfterLayers(t *testing.T) {
	// Listeners must appear after all layer objects in the stream.
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000)

	sm := ab.StateMachine("SM")
	layer := sm.Layer("Layer 1")
	layer.State("idle")
	sm.Listener(shape, builder.ListenerPointerDown)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	const tkStateMachineLayer = 57
	lastLayerIdx := -1
	listenerIdx := -1

	for i, o := range objects {
		if o.TypeKey() == tkStateMachineLayer {
			lastLayerIdx = i
		}
		if o.TypeKey() == tkStateMachineListenerSingle {
			listenerIdx = i
		}
	}

	if listenerIdx < 0 {
		t.Fatal("listener not found in stream")
	}
	if lastLayerIdx < 0 {
		t.Fatal("layer not found in stream")
	}
	if listenerIdx <= lastLayerIdx {
		t.Errorf("listener at index %d must come after layer at index %d", listenerIdx, lastLayerIdx)
	}
}
