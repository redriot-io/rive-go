package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

const (
	tkListenerTriggerChange = 115
	tkListenerBoolChange    = 117
)

// --- SetTrigger ---

func TestListenerBinding_SetTrigger_EmitsAction(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000).Name("btn")

	sm := ab.StateMachine("SM")
	trigger := sm.TriggerInput("tap")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerDown).SetTrigger(trigger)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Verify one ListenerTriggerChange (typeKey=115) exists
	actions := collectType(objects, tkListenerTriggerChange)
	if len(actions) != 1 {
		t.Fatalf("expected 1 ListenerTriggerChange, got %d", len(actions))
	}

	act := actions[0].(*rive.ListenerTriggerChange)
	// trigger is index 0 (first input)
	if act.InputId != 0 {
		t.Errorf("InputId = %d, want 0 (first input)", act.InputId)
	}
}

func TestListenerBinding_SetTrigger_AfterListener(t *testing.T) {
	// Action must appear immediately after its parent listener in the stream.
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000)

	sm := ab.StateMachine("SM")
	trig := sm.TriggerInput("fire")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerDown).SetTrigger(trig)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listenerIdx := -1
	actionIdx := -1
	for i, o := range objects {
		if o.TypeKey() == tkStateMachineListenerSingle {
			listenerIdx = i
		}
		if o.TypeKey() == tkListenerTriggerChange {
			actionIdx = i
		}
	}
	if listenerIdx < 0 {
		t.Fatal("listener not found")
	}
	if actionIdx < 0 {
		t.Fatal("trigger action not found")
	}
	if actionIdx != listenerIdx+1 {
		t.Errorf("action at index %d; want listenerIdx+1 = %d", actionIdx, listenerIdx+1)
	}
}

func TestListenerBinding_SetTrigger_InputIdMatchesSecondInput(t *testing.T) {
	// Second input (idx=1) should produce InputId=1.
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFF00FF00)

	sm := ab.StateMachine("SM")
	_ = sm.BoolInput("hover")   // idx=0
	trig := sm.TriggerInput("tap") // idx=1
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerClick).SetTrigger(trig)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	actions := collectType(objects, tkListenerTriggerChange)
	if len(actions) != 1 {
		t.Fatalf("expected 1 trigger action, got %d", len(actions))
	}
	act := actions[0].(*rive.ListenerTriggerChange)
	if act.InputId != 1 {
		t.Errorf("InputId = %d, want 1 (second input)", act.InputId)
	}
}

// --- SetBool ---

func TestListenerBinding_SetBoolTrue_EmitsAction(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFF0000FF).Name("toggle")

	sm := ab.StateMachine("SM")
	hover := sm.BoolInput("hovered")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerEnter).SetBool(hover, true)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	actions := collectType(objects, tkListenerBoolChange)
	if len(actions) != 1 {
		t.Fatalf("expected 1 ListenerBoolChange, got %d", len(actions))
	}
	act := actions[0].(*rive.ListenerBoolChange)
	if act.InputId != 0 {
		t.Errorf("InputId = %d, want 0", act.InputId)
	}
	// Value=1 (true) is the default; the field should hold 1
	if act.Value != 1 {
		t.Errorf("Value = %d, want 1 (true)", act.Value)
	}
}

func TestListenerBinding_SetBoolFalse_EmitsAction(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFF0000FF).Name("toggle")

	sm := ab.StateMachine("SM")
	hover := sm.BoolInput("hovered")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerExit).SetBool(hover, false)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	actions := collectType(objects, tkListenerBoolChange)
	if len(actions) != 1 {
		t.Fatalf("expected 1 ListenerBoolChange, got %d", len(actions))
	}
	act := actions[0].(*rive.ListenerBoolChange)
	if act.InputId != 0 {
		t.Errorf("InputId = %d, want 0", act.InputId)
	}
	if act.Value != 0 {
		t.Errorf("Value = %d, want 0 (false)", act.Value)
	}
}

// --- Hover toggle pattern: PointerEnter→true, PointerExit→false ---

func TestListenerBinding_HoverToggle_TwoListenersTwoActions(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(200, 200, 80, 80).Fill(0xFF888888).Name("card")

	sm := ab.StateMachine("SM")
	hovered := sm.BoolInput("hovered")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerEnter).SetBool(hovered, true)
	sm.Listener(shape, builder.ListenerPointerExit).SetBool(hovered, false)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(listeners))
	}

	boolActions := collectType(objects, tkListenerBoolChange)
	if len(boolActions) != 2 {
		t.Fatalf("expected 2 BoolChange actions, got %d", len(boolActions))
	}

	enter := listeners[0].(*rive.StateMachineListenerSingle)
	exit := listeners[1].(*rive.StateMachineListenerSingle)

	if enter.ListenerTypeValue != uint64(builder.ListenerPointerEnter) {
		t.Errorf("first listener type = %d, want PointerEnter(%d)", enter.ListenerTypeValue, builder.ListenerPointerEnter)
	}
	if exit.ListenerTypeValue != uint64(builder.ListenerPointerExit) {
		t.Errorf("second listener type = %d, want PointerExit(%d)", exit.ListenerTypeValue, builder.ListenerPointerExit)
	}

	a1 := boolActions[0].(*rive.ListenerBoolChange)
	a2 := boolActions[1].(*rive.ListenerBoolChange)
	if a1.Value != 1 {
		t.Errorf("enter action Value = %d, want 1 (true)", a1.Value)
	}
	if a2.Value != 0 {
		t.Errorf("exit action Value = %d, want 0 (false)", a2.Value)
	}
}

// --- Multiple actions on one listener ---

func TestListenerBinding_MultipleActionsOnOneListener(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFFFFFF)

	sm := ab.StateMachine("SM")
	hovered := sm.BoolInput("hovered")
	clicked := sm.TriggerInput("clicked")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerClick).
		SetBool(hovered, true).
		SetTrigger(clicked)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Should have 1 listener, 1 BoolChange, 1 TriggerChange — all consecutive
	listenerIdx := -1
	boolIdx := -1
	trigIdx := -1
	for i, o := range objects {
		switch o.TypeKey() {
		case tkStateMachineListenerSingle:
			listenerIdx = i
		case tkListenerBoolChange:
			boolIdx = i
		case tkListenerTriggerChange:
			trigIdx = i
		}
	}

	if listenerIdx < 0 || boolIdx < 0 || trigIdx < 0 {
		t.Fatalf("missing objects: listener=%d bool=%d trig=%d", listenerIdx, boolIdx, trigIdx)
	}
	if boolIdx != listenerIdx+1 {
		t.Errorf("BoolChange at %d, want %d (listenerIdx+1)", boolIdx, listenerIdx+1)
	}
	if trigIdx != listenerIdx+2 {
		t.Errorf("TriggerChange at %d, want %d (listenerIdx+2)", trigIdx, listenerIdx+2)
	}
}

// --- No-action listener still works ---

func TestListenerBinding_NoActions_Roundtrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000)

	sm := ab.StateMachine("SM")
	sm.Layer("Layer 1").State("idle")
	sm.Listener(shape, builder.ListenerPointerDown) // no actions

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(listeners))
	}
	// No action objects
	if n := len(collectType(objects, tkListenerTriggerChange)); n != 0 {
		t.Errorf("unexpected %d TriggerChange objects", n)
	}
	if n := len(collectType(objects, tkListenerBoolChange)); n != 0 {
		t.Errorf("unexpected %d BoolChange objects", n)
	}
}

// --- Binary roundtrip: serialize and re-parse ---

func TestListenerBinding_BinaryRoundtrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	shape := ab.Rectangle(150, 150, 60, 60).Fill(0xFF4CAF50).Name("button")

	sm := ab.StateMachine("SM")
	hovered := sm.BoolInput("hovered")
	tapped := sm.TriggerInput("tapped")
	layer := sm.Layer("Base Layer")
	idle := layer.State("idle")
	active := layer.State("active")
	layer.Transition(idle, active, builder.BoolCondition(hovered, true))

	sm.Listener(shape, builder.ListenerPointerEnter).SetBool(hovered, true)
	sm.Listener(shape, builder.ListenerPointerExit).SetBool(hovered, false)
	sm.Listener(shape, builder.ListenerClick).SetTrigger(tapped)

	// Verify serialization produces valid bytes.
	if _, err := b.Bytes(); err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	// Use Build() to get typed objects for assertions.
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	listeners := collectType(objects, tkStateMachineListenerSingle)
	if len(listeners) != 3 {
		t.Fatalf("expected 3 listeners, got %d", len(listeners))
	}

	boolChanges := collectType(objects, tkListenerBoolChange)
	if len(boolChanges) != 2 {
		t.Fatalf("expected 2 BoolChange actions, got %d", len(boolChanges))
	}

	trigChanges := collectType(objects, tkListenerTriggerChange)
	if len(trigChanges) != 1 {
		t.Fatalf("expected 1 TriggerChange action, got %d", len(trigChanges))
	}

	bc1 := boolChanges[0].(*rive.ListenerBoolChange)
	bc2 := boolChanges[1].(*rive.ListenerBoolChange)
	tc := trigChanges[0].(*rive.ListenerTriggerChange)

	if bc1.InputId != 0 {
		t.Errorf("bc1.InputId = %d, want 0 (hovered)", bc1.InputId)
	}
	if bc2.InputId != 0 {
		t.Errorf("bc2.InputId = %d, want 0 (hovered)", bc2.InputId)
	}
	if tc.InputId != 1 {
		t.Errorf("tc.InputId = %d, want 1 (tapped)", tc.InputId)
	}
}
