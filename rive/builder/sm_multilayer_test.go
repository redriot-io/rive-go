package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive/builder"
)

// TestMultiLayer_TwoLayers verifies that two Layer() calls produce two
// StateMachineLayer objects.
func TestMultiLayer_TwoLayers(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("anim_a", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("anim_b", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 100.0)

	sm := ab.StateMachine("SM")
	active := sm.BoolInput("active")

	layer1 := sm.Layer("Layer1")
	s1a := layer1.State("Off", builder.WithAnimation("anim_a"))
	s1b := layer1.State("On", builder.WithAnimation("anim_b"))
	layer1.Transition(s1a, s1b).When(active).IsTrue()
	layer1.Transition(s1b, s1a).When(active).IsFalse()

	layer2 := sm.Layer("Layer2")
	s2a := layer2.State("A", builder.WithAnimation("anim_a"))
	s2b := layer2.State("B", builder.WithAnimation("anim_b"))
	layer2.Transition(s2a, s2b).When(active).IsTrue()
	layer2.Transition(s2b, s2a).When(active).IsFalse()

	_ = s1a; _ = s1b; _ = s2a; _ = s2b

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Two StateMachineLayer (typeKey=57)
	if n := countType(f.Objects, 57); n != 2 {
		t.Errorf("want 2 StateMachineLayer (typeKey 57), got %d", n)
	}
}

// TestMultiLayer_EachLayerHasOwnSentinels verifies each layer emits its own
// AnyState, EntryState, ExitState sentinels.
func TestMultiLayer_EachLayerHasOwnSentinels(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("a", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)

	sm := ab.StateMachine("SM")
	sm.Layer("L1").State("S1", builder.WithAnimation("a"))
	sm.Layer("L2").State("S2", builder.WithAnimation("a"))

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Two layers → 2× each sentinel
	if n := countType(f.Objects, 62); n != 2 { // AnyState
		t.Errorf("want 2 AnyState, got %d", n)
	}
	if n := countType(f.Objects, 63); n != 2 { // EntryState
		t.Errorf("want 2 EntryState, got %d", n)
	}
	if n := countType(f.Objects, 64); n != 2 { // ExitState
		t.Errorf("want 2 ExitState, got %d", n)
	}
}

// TestExitState_SentinelAtIndex2 verifies ExitState is emitted as the 3rd
// layer child (index 2), between EntryState (index 1) and user states (index 3+).
func TestExitState_SentinelAtIndex2(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("anim", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)

	sm := ab.StateMachine("SM")
	sm.Layer("L").State("S", builder.WithAnimation("anim"))

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Find StateMachineLayer and verify the object after EntryState is ExitState (64),
	// and the object after ExitState is AnimationState (61).
	for i, o := range f.Objects {
		if o.TypeKey() != 63 { // EntryState
			continue
		}
		// [i] = EntryState
		// [i+1] = StateTransition (entry auto-transition)
		// [i+2] = ExitState (3rd sentinel)
		// [i+3] = AnimationState (first user state, child index 3)
		if i+3 >= len(f.Objects) {
			t.Fatal("not enough objects after EntryState")
		}
		if f.Objects[i+2].TypeKey() != 64 {
			t.Errorf("expected ExitState (64) at +2 from EntryState, got typeKey %d",
				f.Objects[i+2].TypeKey())
		}
		if f.Objects[i+3].TypeKey() != 61 { // AnimationState
			t.Errorf("expected AnimationState (61) at +3 from EntryState, got typeKey %d",
				f.Objects[i+3].TypeKey())
		}
		break
	}
}

// TestExitState_UserStateIndex3 verifies first user state has layer-child index 3.
func TestExitState_UserStateIndex3(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("anim", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)

	sm := ab.StateMachine("SM")
	layer := sm.Layer("L")
	layer.State("S", builder.WithAnimation("anim"))

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// EntryState's auto-transition stateTo must be 3 (first user state at child 3)
	for i, o := range f.Objects {
		if o.TypeKey() != 63 { // EntryState
			continue
		}
		if i+1 >= len(f.Objects) || f.Objects[i+1].TypeKey() != 65 {
			t.Fatal("EntryState not followed by StateTransition")
		}
		props := propsByKey(f.Objects[i+1].Properties())
		if v, ok := props[151]; !ok || v.Value.(uint64) != 3 {
			t.Errorf("EntryState stateTo = %v, want 3", props[151].Value)
		}
		break
	}
}

// TestExitState_TransitionTo verifies that a transition to LayerBuilder.ExitState()
// emits a StateTransition with stateTo=2.
func TestExitState_TransitionTo(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("anim", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)

	sm := ab.StateMachine("SM")
	done := sm.BoolInput("done")
	layer := sm.Layer("L")
	play := layer.State("Play", builder.WithAnimation("anim"))
	layer.Transition(play, layer.ExitState()).When(done).IsTrue()

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Find a StateTransition with stateTo=2 (ExitState) that has a condition
	found := false
	for _, o := range f.Objects {
		if o.TypeKey() != 65 { // StateTransition
			continue
		}
		props := propsByKey(o.Properties())
		if v, ok := props[151]; ok && v.Value.(uint64) == 2 {
			found = true
		}
	}
	if !found {
		t.Error("StateTransition with stateTo=2 (ExitState) not found")
	}
}

// TestMultiLayer_SharedInputs verifies both layers use the same input index.
func TestMultiLayer_SharedInputs(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("a_off", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("a_on", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 200.0)
	ab.Animation("b_off", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropY, 0, 0.0)
	ab.Animation("b_on", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropY, 0, 200.0)

	sm := ab.StateMachine("SM")
	active := sm.BoolInput("active") // input index 0

	layer1 := sm.Layer("L1")
	l1off := layer1.State("Off", builder.WithAnimation("a_off"))
	l1on := layer1.State("On", builder.WithAnimation("a_on"))
	layer1.Transition(l1off, l1on).When(active).IsTrue()

	layer2 := sm.Layer("L2")
	l2off := layer2.State("Off", builder.WithAnimation("b_off"))
	l2on := layer2.State("On", builder.WithAnimation("b_on"))
	layer2.Transition(l2off, l2on).When(active).IsTrue()

	_ = l1on; _ = l2on

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Both layers should have TransitionBoolCondition with InputId=0 (same "active" input)
	condCount := 0
	for _, o := range f.Objects {
		if o.TypeKey() != 71 { // TransitionBoolCondition
			continue
		}
		props := propsByKey(o.Properties())
		if v, ok := props[155]; ok && v.Value.(uint64) == 0 {
			condCount++
		}
	}
	if condCount != 2 {
		t.Errorf("want 2 TransitionBoolCondition with InputId=0 (shared input), got %d", condCount)
	}
}

// TestMultiLayer_RoundTrip verifies a multi-layer SM round-trips through ReadBytes.
func TestMultiLayer_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 420, 200)
	rect := ab.Rectangle(60, 100, 80, 80).Fill(0xFF1565C0)

	ab.Animation("slow", builder.WithDuration(60), builder.WithLoop(builder.PingPong)).
		KeyframeFloat(rect, builder.PropX, 0, 60.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropX, 60, 200.0, builder.Linear())
	ab.Animation("fast", builder.WithDuration(60), builder.WithLoop(builder.PingPong)).
		KeyframeFloat(rect, builder.PropX, 0, 60.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropX, 60, 360.0, builder.Linear())
	ab.Animation("blue", builder.WithDuration(2)).
		KeyframeColor(rect, builder.PropColorValue, 0, 0xFF1565C0, builder.Linear())
	ab.Animation("orange", builder.WithDuration(2)).
		KeyframeColor(rect, builder.PropColorValue, 0, 0xFFE65100, builder.Linear())

	sm := ab.StateMachine("MultiSM")
	running := sm.BoolInput("running")
	hot := sm.BoolInput("hot")

	layer1 := sm.Layer("Position")
	slow := layer1.State("Slow", builder.WithAnimation("slow"))
	fast := layer1.State("Fast", builder.WithAnimation("fast"))
	layer1.Transition(slow, fast).When(running).IsTrue()
	layer1.Transition(fast, slow).When(running).IsFalse()

	layer2 := sm.Layer("Color")
	blue := layer2.State("Blue", builder.WithAnimation("blue"))
	orange := layer2.State("Orange", builder.WithAnimation("orange"))
	layer2.Transition(blue, orange).When(hot).IsTrue()
	layer2.Transition(orange, blue).When(hot).IsFalse()

	_ = fast; _ = orange

	data := mustBuild(t, b)
	if _, err := mustReadBytes(t, data), false; err {
		t.Fatal("round-trip ReadBytes failed")
	}
}
