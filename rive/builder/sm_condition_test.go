package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive/builder"
)

// TestFluent_BoolIsTrue verifies .When(input).IsTrue() emits TransitionBoolCondition with OpValue=1.
func TestFluent_BoolIsTrue(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	active := sm.BoolInput("active")
	layer := sm.Layer("L")
	idle := layer.State("Idle")
	on := layer.State("On")
	layer.Transition(idle, on).When(active).IsTrue()

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	cond := findType(f.Objects, 71) // TransitionBoolCondition
	if cond == nil {
		t.Fatal("TransitionBoolCondition (typeKey 71) not found")
	}
	props := propsByKey(cond.Properties())
	// OpValue=1 (true); key 156 must be present (default is 0)
	if v, ok := props[156]; !ok || v.Value.(uint64) != 1 {
		t.Errorf("TransitionBoolCondition.OpValue = %v, want 1", props[156].Value)
	}
}

// TestFluent_BoolIsFalse verifies .When(input).IsFalse() suppresses OpValue (default=0).
func TestFluent_BoolIsFalse(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	active := sm.BoolInput("active")
	layer := sm.Layer("L")
	on := layer.State("On")
	idle := layer.State("Idle")
	layer.Transition(on, idle).When(active).IsFalse()

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	cond := findType(f.Objects, 71)
	if cond == nil {
		t.Fatal("TransitionBoolCondition (typeKey 71) not found")
	}
	props := propsByKey(cond.Properties())
	// OpValue=0 (false) is the default and must be suppressed
	if _, ok := props[156]; ok {
		t.Error("TransitionBoolCondition.OpValue key 156 should be suppressed for false (default=0)")
	}
}

// TestFluent_WhenTrigger verifies .WhenTrigger(input) emits TransitionTriggerCondition.
func TestFluent_WhenTrigger(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	fire := sm.TriggerInput("fire")
	layer := sm.Layer("L")
	idle := layer.State("Idle")
	burst := layer.State("Burst")
	layer.Transition(idle, burst).WhenTrigger(fire)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if findType(f.Objects, 68) == nil { // TransitionTriggerCondition
		t.Fatal("TransitionTriggerCondition (typeKey 68) not found")
	}
}

// TestFluent_NumberEquals verifies .When(input).Equals(v) emits TransitionNumberCondition with op=0.
func TestFluent_NumberEquals(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	slow := layer.State("Slow")
	fast := layer.State("Fast")
	layer.Transition(slow, fast).When(speed).Equals(5.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	cond := findType(f.Objects, 70) // TransitionNumberCondition
	if cond == nil {
		t.Fatal("TransitionNumberCondition (typeKey 70) not found")
	}
	props := propsByKey(cond.Properties())
	// OpValue=0 (Equal) is the default and must be suppressed
	if _, ok := props[156]; ok {
		t.Errorf("OpValue key 156 should be suppressed for Equal (default=0)")
	}
	// Value=5.0 must be emitted at key 157
	if v, ok := props[157]; !ok || v.Value.(float64) != 5.0 {
		t.Errorf("TransitionNumberCondition.Value = %v, want 5.0", props[157].Value)
	}
}

// TestFluent_NumberGreaterThan verifies .When(input).GreaterThan(v) uses op=3.
func TestFluent_NumberGreaterThan(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	slow := layer.State("Slow")
	fast := layer.State("Fast")
	layer.Transition(slow, fast).When(speed).GreaterThan(10.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	cond := findType(f.Objects, 70)
	if cond == nil {
		t.Fatal("TransitionNumberCondition (typeKey 70) not found")
	}
	props := propsByKey(cond.Properties())
	if v, ok := props[156]; !ok || v.Value.(uint64) != 3 {
		t.Errorf("GreaterThan OpValue = %v, want 3", props[156].Value)
	}
}

// TestFluent_NumberLessThan verifies .When(input).LessThan(v) uses op=2.
func TestFluent_NumberLessThan(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	speed := sm.NumberInput("speed")
	layer := sm.Layer("L")
	fast := layer.State("Fast")
	slow := layer.State("Slow")
	layer.Transition(fast, slow).When(speed).LessThan(3.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	cond := findType(f.Objects, 70)
	if cond == nil {
		t.Fatal("TransitionNumberCondition (typeKey 70) not found")
	}
	props := propsByKey(cond.Properties())
	if v, ok := props[156]; !ok || v.Value.(uint64) != 2 {
		t.Errorf("LessThan OpValue = %v, want 2", props[156].Value)
	}
}

// TestFluent_AndConditions verifies multiple .When() calls create ANDed conditions.
func TestFluent_AndConditions(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	pressed := sm.BoolInput("pressed")
	hovered := sm.BoolInput("hovered")
	layer := sm.Layer("L")
	press := layer.State("Press")
	hover := layer.State("Hover")
	layer.Transition(press, hover).
		When(pressed).IsFalse().
		When(hovered).IsTrue()

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Two AND conditions → two TransitionBoolCondition objects
	if n := countType(f.Objects, 71); n != 2 {
		t.Errorf("want 2 TransitionBoolCondition (AND), got %d", n)
	}
}

// TestFluent_Duration verifies .Duration(ms) sets StateTransition.Duration (key 158).
func TestFluent_Duration(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	layer := sm.Layer("L")
	idle := layer.State("Idle")
	active := layer.State("Active")
	layer.Transition(idle, active).Duration(200)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	found := false
	for _, o := range f.Objects {
		if o.TypeKey() != 65 { // StateTransition
			continue
		}
		if v, ok := propsByKey(o.Properties())[158]; ok && v.Value.(uint64) == 200 {
			found = true
		}
	}
	if !found {
		t.Error("StateTransition with Duration=200 (key 158) not found")
	}
}

// TestFluent_ExitTime verifies .ExitTime(frames) sets StateTransition.ExitTime (key 160).
func TestFluent_ExitTime(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	layer := sm.Layer("L")
	idle := layer.State("Idle")
	active := layer.State("Active")
	layer.Transition(idle, active).ExitTime(15)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	found := false
	for _, o := range f.Objects {
		if o.TypeKey() != 65 {
			continue
		}
		if v, ok := propsByKey(o.Properties())[160]; ok && v.Value.(uint64) == 15 {
			found = true
		}
	}
	if !found {
		t.Error("StateTransition with ExitTime=15 (key 160) not found")
	}
}

// TestFluent_DurationAndCondition verifies Duration + condition can be chained together.
func TestFluent_DurationAndCondition(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	done := sm.BoolInput("done")
	layer := sm.Layer("L")
	play := layer.State("Play")
	end := layer.State("End")
	layer.Transition(play, end).Duration(100).When(done).IsTrue()

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Duration=100 on the transition
	foundDur := false
	for _, o := range f.Objects {
		if o.TypeKey() != 65 {
			continue
		}
		if v, ok := propsByKey(o.Properties())[158]; ok && v.Value.(uint64) == 100 {
			foundDur = true
		}
	}
	if !foundDur {
		t.Error("StateTransition with Duration=100 (key 158) not found")
	}

	// Bool condition present
	if findType(f.Objects, 71) == nil {
		t.Error("TransitionBoolCondition (typeKey 71) not found after Duration chain")
	}
}
