package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive/builder"
)

const (
	tkBlendState1DInput = uint32(76)
	tkBlendAnimation1D  = uint32(75)
)

// TestBlend1D_BasicEmission verifies that BlendState1DInput (76) and
// BlendAnimation1D (75) objects are emitted.
func TestBlend1D_BasicEmission(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("walk", builder.WithDuration(60)).
		KeyframeFloat(rect, builder.PropX, 0, 0.0).
		KeyframeFloat(rect, builder.PropX, 60, 200.0)
	ab.Animation("run", builder.WithDuration(60)).
		KeyframeFloat(rect, builder.PropX, 0, 0.0).
		KeyframeFloat(rect, builder.PropX, 60, 400.0)

	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	blend := layer.BlendState1D("SpeedBlend", speed)
	blend.AddAnimation("walk", 0.0).AddAnimation("run", 1.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if findType(f.Objects, tkBlendState1DInput) == nil {
		t.Fatal("BlendState1DInput (typeKey 76) not found")
	}
	if n := countType(f.Objects, tkBlendAnimation1D); n != 2 {
		t.Errorf("want 2 BlendAnimation1D (typeKey 75), got %d", n)
	}
}

// TestBlend1D_InputId verifies InputId (key 167) matches the numeric input index.
func TestBlend1D_InputId(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("a", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("bb", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 100.0)

	sm := ab.StateMachine("SM")
	_ = sm.BoolInput("first") // input index 0
	speed := sm.NumberInput("speed") // input index 1
	layer := sm.Layer("L")
	blend := layer.BlendState1D("B", speed)
	blend.AddAnimation("a", 0.0).AddAnimation("bb", 1.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	bs := findType(f.Objects, tkBlendState1DInput)
	if bs == nil {
		t.Fatal("BlendState1DInput not found")
	}
	props := propsByKey(bs.Properties())
	// InputId=1 (speed is second input, index 1); key 167
	if v, ok := props[167]; !ok || v.Value.(uint64) != 1 {
		t.Errorf("BlendState1DInput.InputId = %v, want 1", props[167].Value)
	}
}

// TestBlend1D_ThresholdValues verifies BlendAnimation1D.Value (key 166) thresholds.
func TestBlend1D_ThresholdValues(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("walk", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("jog", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 100.0)
	ab.Animation("run", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 200.0)

	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	blend := layer.BlendState1D("B", speed)
	blend.AddAnimation("walk", 0.0).
		AddAnimation("jog", 0.5).
		AddAnimation("run", 1.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	bAnims := collectType(f.Objects, tkBlendAnimation1D)
	if len(bAnims) != 3 {
		t.Fatalf("want 3 BlendAnimation1D, got %d", len(bAnims))
	}

	// threshold=0 → Value suppressed (default=0)
	props0 := propsByKey(bAnims[0].Properties())
	if _, ok := props0[166]; ok {
		t.Error("walk BlendAnimation1D.Value (key 166) should be suppressed (threshold=0)")
	}

	// threshold=0.5 → Value=0.5 emitted at key 166
	props1 := propsByKey(bAnims[1].Properties())
	if v, ok := props1[166]; !ok || v.Value.(float64) != 0.5 {
		t.Errorf("jog BlendAnimation1D.Value = %v, want 0.5", props1[166].Value)
	}

	// threshold=1.0 → Value=1.0 emitted
	props2 := propsByKey(bAnims[2].Properties())
	if v, ok := props2[166]; !ok || v.Value.(float64) != 1.0 {
		t.Errorf("run BlendAnimation1D.Value = %v, want 1.0", props2[166].Value)
	}
}

// TestBlend1D_AnimationIds verifies BlendAnimation1D.AnimationId (key 165) maps correctly.
func TestBlend1D_AnimationIds(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	// walk=index 0, run=index 1
	ab.Animation("walk", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("run", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 300.0)

	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	blend := layer.BlendState1D("B", speed)
	blend.AddAnimation("walk", 0.0).AddAnimation("run", 1.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	bAnims := collectType(f.Objects, tkBlendAnimation1D)
	if len(bAnims) != 2 {
		t.Fatalf("want 2 BlendAnimation1D, got %d", len(bAnims))
	}

	// walk → animationId=0
	props0 := propsByKey(bAnims[0].Properties())
	if v, ok := props0[165]; !ok || v.Value.(uint64) != 0 {
		t.Errorf("walk BlendAnimation1D.AnimationId = %v, want 0", props0[165].Value)
	}

	// run → animationId=1
	props1 := propsByKey(bAnims[1].Properties())
	if v, ok := props1[165]; !ok || v.Value.(uint64) != 1 {
		t.Errorf("run BlendAnimation1D.AnimationId = %v, want 1", props1[165].Value)
	}
}

// TestBlend1D_EntryTransitionIndex verifies EntryState auto-transition points to
// the BlendState1DInput at layer-child index 2 (AnyState=0, EntryState=1, first user state=2).
func TestBlend1D_EntryTransitionIndex(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("walk", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("run", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 300.0)

	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	blend := layer.BlendState1D("B", speed)
	blend.AddAnimation("walk", 0.0).AddAnimation("run", 1.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Find the EntryState's StateTransition and check stateTo=3
	entryFound := false
	for i, o := range f.Objects {
		if o.TypeKey() != 63 { // EntryState
			continue
		}
		entryFound = true
		// Next object should be StateTransition (65)
		if i+1 >= len(f.Objects) || f.Objects[i+1].TypeKey() != 65 {
			t.Error("EntryState not followed by StateTransition")
			break
		}
		props := propsByKey(f.Objects[i+1].Properties())
		// stateTo key 151
		if v, ok := props[151]; !ok || v.Value.(uint64) != 3 {
			t.Errorf("EntryState transition stateTo = %v, want 3", props[151].Value)
		}
		break
	}
	if !entryFound {
		t.Error("EntryState (typeKey 63) not found")
	}
}

// TestBlend1D_InitialValue verifies that InputRef.WithValue sets StateMachineNumber.Value.
func TestBlend1D_InitialValue(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("a", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 0.0)
	ab.Animation("c", builder.WithDuration(30)).KeyframeFloat(rect, builder.PropX, 0, 100.0)

	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed").WithValue(0.5)
	layer := sm.Layer("L")
	blend := layer.BlendState1D("B", speed)
	blend.AddAnimation("a", 0.0).AddAnimation("c", 1.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// StateMachineNumber (typeKey 56), Value key 140
	numInput := findType(f.Objects, 56)
	if numInput == nil {
		t.Fatal("StateMachineNumber (typeKey 56) not found")
	}
	props := propsByKey(numInput.Properties())
	if v, ok := props[140]; !ok || v.Value.(float64) != 0.5 {
		t.Errorf("StateMachineNumber.Value = %v, want 0.5", props[140].Value)
	}
}

// TestBlend1D_RoundTrip verifies the .riv round-trips through ReadBytes.
func TestBlend1D_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 300)

	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	ab.Animation("slow", builder.WithDuration(60), builder.WithLoop(builder.PingPong)).
		KeyframeFloat(rect, builder.PropX, 0, 50.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropX, 60, 200.0, builder.Linear())
	ab.Animation("fast", builder.WithDuration(60), builder.WithLoop(builder.PingPong)).
		KeyframeFloat(rect, builder.PropX, 0, 50.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropX, 60, 380.0, builder.Linear())

	sm := ab.StateMachine("SpeedSM")
	speed := sm.NumberInput("speed").WithValue(0.5)
	layer := sm.Layer("Main")
	blend := layer.BlendState1D("SpeedBlend", speed)
	blend.AddAnimation("slow", 0.0).AddAnimation("fast", 1.0)

	data := mustBuild(t, b)
	if _, err := mustReadBytes(t, data), false; err {
		t.Fatal("round-trip ReadBytes failed")
	}
}
