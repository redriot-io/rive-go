package builder

import (
	"fmt"

	"github.com/redriot-io/rive-go/rive"
)

// inputKind enumerates state machine input types.
type inputKind uint8

const (
	inputBool    inputKind = iota
	inputNumber  inputKind = iota
	inputTrigger inputKind = iota
)

// InputRef is a handle to a state machine input, used in transition conditions.
type InputRef struct {
	kind inputKind
	name string
	idx  uint64 // global emission index, set during emit()
}

// CompareOp is the comparison operator for number conditions.
type CompareOp uint8

const (
	Equal              CompareOp = 0
	NotEqual           CompareOp = 1
	LessThan           CompareOp = 2
	GreaterThan        CompareOp = 3
	LessThanOrEqual    CompareOp = 4
	GreaterThanOrEqual CompareOp = 5
)

// Condition is a predicate on a state machine transition.
type Condition interface {
	makeConditionObject(inputIdx uint64) rive.Object
	inputRef() *InputRef
}

// BoolCond fires when the named input equals value.
type BoolCond struct {
	ref   *InputRef
	value bool
}

func (c BoolCond) inputRef() *InputRef { return c.ref }
func (c BoolCond) makeConditionObject(inputIdx uint64) rive.Object {
	obj := &rive.TransitionBoolCondition{}
	obj.InputId = inputIdx
	if c.value {
		obj.OpValue = 1 // "equal to true"
	}
	return obj
}

// TriggerCond fires when the trigger input is activated.
type TriggerCond struct{ ref *InputRef }

func (c TriggerCond) inputRef() *InputRef { return c.ref }
func (c TriggerCond) makeConditionObject(inputIdx uint64) rive.Object {
	obj := &rive.TransitionTriggerCondition{}
	obj.InputId = inputIdx
	return obj
}

// NumberCond fires when the number input satisfies the comparison.
type NumberCond struct {
	ref   *InputRef
	op    CompareOp
	value float64
}

func (c NumberCond) inputRef() *InputRef { return c.ref }
func (c NumberCond) makeConditionObject(inputIdx uint64) rive.Object {
	obj := &rive.TransitionNumberCondition{}
	obj.InputId = inputIdx
	obj.OpValue = uint64(c.op)
	obj.Value = c.value
	return obj
}

// BoolCondition returns a condition that fires when input == value.
func BoolCondition(input *InputRef, value bool) Condition {
	return BoolCond{ref: input, value: value}
}

// TriggerCondition returns a condition that fires when the trigger fires.
func TriggerCondition(input *InputRef) Condition {
	return TriggerCond{ref: input}
}

// NumberCondition returns a condition based on a number comparison.
func NumberCondition(input *InputRef, op CompareOp, value float64) Condition {
	return NumberCond{ref: input, op: op, value: value}
}

// StateOption configures a state.
type StateOption func(*stateConfig)

type stateConfig struct {
	animName string
}

// WithAnimation links this state to a named animation.
func WithAnimation(animName string) StateOption {
	return func(c *stateConfig) { c.animName = animName }
}

// StateRef is a handle to a state in a layer.
type StateRef struct {
	name     string
	animName string
	idx      uint64 // global emission index, set during emit
}

type transitionEntry struct {
	to             *StateRef
	conditions     []Condition
	durationMs     int
	exitTimeFrames int
}

type stateEntry struct {
	ref         *StateRef
	transitions []*transitionEntry
}

// TransitionRef is a handle to a single state machine transition.
// Use When/WhenTrigger to add conditions (ANDed together) and Duration/ExitTime
// to configure timing. Methods return the receiver for chaining.
type TransitionRef struct {
	entry *transitionEntry
}

// When returns a ConditionBuilder for a bool or number input.
// Call IsTrue/IsFalse for bool inputs or Equals/GreaterThan/LessThan for number inputs.
// Multiple When calls are ANDed.
func (t *TransitionRef) When(input *InputRef) *ConditionBuilder {
	return &ConditionBuilder{tr: t, input: input}
}

// WhenTrigger adds a trigger condition and returns t for further chaining.
func (t *TransitionRef) WhenTrigger(input *InputRef) *TransitionRef {
	t.entry.conditions = append(t.entry.conditions, TriggerCond{ref: input})
	return t
}

// Duration sets the blend duration in milliseconds.
func (t *TransitionRef) Duration(ms int) *TransitionRef {
	t.entry.durationMs = ms
	return t
}

// ExitTime sets the number of frames the source animation must complete before
// the transition is allowed to fire.
func (t *TransitionRef) ExitTime(frames int) *TransitionRef {
	t.entry.exitTimeFrames = frames
	return t
}

// ConditionBuilder completes a single condition started by TransitionRef.When.
type ConditionBuilder struct {
	tr    *TransitionRef
	input *InputRef
}

// IsTrue adds a bool-is-true condition and returns the TransitionRef for chaining.
func (c *ConditionBuilder) IsTrue() *TransitionRef {
	c.tr.entry.conditions = append(c.tr.entry.conditions, BoolCond{ref: c.input, value: true})
	return c.tr
}

// IsFalse adds a bool-is-false condition and returns the TransitionRef for chaining.
func (c *ConditionBuilder) IsFalse() *TransitionRef {
	c.tr.entry.conditions = append(c.tr.entry.conditions, BoolCond{ref: c.input, value: false})
	return c.tr
}

// Equals adds a number-equals condition (op=0) and returns the TransitionRef.
func (c *ConditionBuilder) Equals(v float64) *TransitionRef {
	c.tr.entry.conditions = append(c.tr.entry.conditions, NumberCond{ref: c.input, op: Equal, value: v})
	return c.tr
}

// NotEqualTo adds a number-not-equal condition (op=1) and returns the TransitionRef.
func (c *ConditionBuilder) NotEqualTo(v float64) *TransitionRef {
	c.tr.entry.conditions = append(c.tr.entry.conditions, NumberCond{ref: c.input, op: NotEqual, value: v})
	return c.tr
}

// GreaterThan adds a number-greater-than condition (op=3) and returns the TransitionRef.
func (c *ConditionBuilder) GreaterThan(v float64) *TransitionRef {
	c.tr.entry.conditions = append(c.tr.entry.conditions, NumberCond{ref: c.input, op: GreaterThan, value: v})
	return c.tr
}

// LessThan adds a number-less-than condition (op=2) and returns the TransitionRef.
func (c *ConditionBuilder) LessThan(v float64) *TransitionRef {
	c.tr.entry.conditions = append(c.tr.entry.conditions, NumberCond{ref: c.input, op: LessThan, value: v})
	return c.tr
}

// LayerBuilder builds a single state machine layer.
type LayerBuilder struct {
	name   string
	states []*stateEntry
	// anyTransitions are transitions from AnyState
	anyTransitions []*transitionEntry
}

// State adds a named state, optionally linked to an animation.
func (l *LayerBuilder) State(name string, opts ...StateOption) *StateRef {
	cfg := &stateConfig{}
	for _, o := range opts {
		o(cfg)
	}
	ref := &StateRef{name: name, animName: cfg.animName}
	l.states = append(l.states, &stateEntry{ref: ref})
	return ref
}

// Transition adds a transition from → to, optionally with initial conditions.
// Returns a TransitionRef for fluent condition/timing configuration.
// from must be a StateRef returned by this layer's State() method.
func (l *LayerBuilder) Transition(from, to *StateRef, conditions ...Condition) *TransitionRef {
	te := &transitionEntry{to: to, conditions: conditions}
	for _, se := range l.states {
		if se.ref == from {
			se.transitions = append(se.transitions, te)
			return &TransitionRef{entry: te}
		}
	}
	// from not found — return orphan ref (ignored during emit)
	return &TransitionRef{entry: te}
}

// StateMachineBuilder builds a state machine and all its sub-objects.
type StateMachineBuilder struct {
	name      string
	inputs    []*InputRef
	layers    []*LayerBuilder
	listeners []listenerConfig
}

// BoolInput adds a boolean input.
func (sm *StateMachineBuilder) BoolInput(name string) *InputRef {
	ref := &InputRef{kind: inputBool, name: name}
	sm.inputs = append(sm.inputs, ref)
	return ref
}

// NumberInput adds a number input.
func (sm *StateMachineBuilder) NumberInput(name string) *InputRef {
	ref := &InputRef{kind: inputNumber, name: name}
	sm.inputs = append(sm.inputs, ref)
	return ref
}

// TriggerInput adds a trigger input.
func (sm *StateMachineBuilder) TriggerInput(name string) *InputRef {
	ref := &InputRef{kind: inputTrigger, name: name}
	sm.inputs = append(sm.inputs, ref)
	return ref
}

// Layer adds a layer to the state machine.
func (sm *StateMachineBuilder) Layer(name string) *LayerBuilder {
	lb := &LayerBuilder{name: name}
	sm.layers = append(sm.layers, lb)
	return lb
}

// Listener registers a pointer-event listener on target for the given event type.
// The returned ListenerRef is reserved for future action binding (Phase 2C).
func (sm *StateMachineBuilder) Listener(target *ShapeRef, lt ListenerType) *ListenerRef {
	cfg := listenerConfig{target: target, lt: lt}
	sm.listeners = append(sm.listeners, cfg)
	return &ListenerRef{cfg: &sm.listeners[len(sm.listeners)-1]}
}

// preComputeStateIndices assigns indices to inputs and states.
// Inputs are numbered 0,1,2,… within the SM's input list.
// States use layer-child indices: AnyState=0, EntryState=1, first user state=2, …
// The Rive runtime resolves StateToId as a layer-child index, not a user-state-list index.
func (sm *StateMachineBuilder) preComputeStateIndices() {
	for i, inp := range sm.inputs {
		inp.idx = uint64(i)
	}
	for _, layer := range sm.layers {
		for i, se := range layer.states {
			// +2: AnyState is emitted as child 0, EntryState as child 1.
			// TODO(phase3): If ExitState (typeKey=64) is ever emitted as a third
			// sentinel child before user states, this offset must become +3.
			// Currently we emit: AnyState(0) + EntryState(1) = 2 sentinels.
			se.ref.idx = uint64(i + 2)
		}
	}
}

// emit writes all state machine objects into the slice.
// anims provides the lookup table for animationId resolution.
func (sm *StateMachineBuilder) emit(objects *[]rive.Object, anims []*AnimationBuilder) error {
	sm.preComputeStateIndices()

	// Map from animation name → 0-based position in animation list.
	// The Rive runtime's AnimationState.AnimationId is the 0-based animation index.
	animByName := make(map[string]uint64, len(anims))
	for i, a := range anims {
		animByName[a.name] = uint64(i)
	}

	// --- StateMachine ---
	smObj := &rive.StateMachine{}
	smObj.Name = sm.name
	*objects = append(*objects, smObj)

	// --- Inputs ---
	for _, inp := range sm.inputs {
		var obj rive.Object
		switch inp.kind {
		case inputBool:
			o := &rive.StateMachineBool{}
			o.Name = inp.name
			obj = o
		case inputNumber:
			o := &rive.StateMachineNumber{}
			o.Name = inp.name
			obj = o
		case inputTrigger:
			o := &rive.StateMachineTrigger{}
			o.Name = inp.name
			obj = o
		}
		*objects = append(*objects, obj)
	}

	// --- Layers ---
	for _, layer := range sm.layers {
		layerObj := &rive.StateMachineLayer{}
		layerObj.Name = layer.name
		*objects = append(*objects, layerObj)

		// AnyState sentinel (no transitions in this builder)
		*objects = append(*objects, &rive.AnyState{})

		// EntryState + auto transition to first user state
		*objects = append(*objects, &rive.EntryState{})
		if len(layer.states) > 0 {
			entryTrans := newStateTransition(layer.states[0].ref.idx)
			*objects = append(*objects, entryTrans)
		}

		// User states with their transitions (interleaved)
		for _, se := range layer.states {
			var stateObj rive.Object
			if se.ref.animName != "" {
				animIdx, ok := animByName[se.ref.animName]
				if !ok {
					return fmt.Errorf("builder: state %q references unknown animation %q", se.ref.name, se.ref.animName)
				}
				as := &rive.AnimationState{}
				as.Speed = 1.0 // runtime default; zero value would emit Speed=0
				as.AnimationId = animIdx
				stateObj = as
			} else {
				as := &rive.AnimationState{}
				as.Speed = 1.0 // runtime default; zero value would emit Speed=0
				as.AnimationId = ^uint64(0) // sentinel: suppress emission
				stateObj = as
			}
			*objects = append(*objects, stateObj)

			// Transitions from this state
			for _, te := range se.transitions {
				t := newStateTransition(te.to.idx)
				if te.durationMs > 0 {
					t.Duration = uint64(te.durationMs)
				}
				if te.exitTimeFrames > 0 {
					t.ExitTime = uint64(te.exitTimeFrames)
				}
				*objects = append(*objects, t)

				// Conditions under this transition
				for _, cond := range te.conditions {
					inp := cond.inputRef()
					condObj := cond.makeConditionObject(inp.idx)
					*objects = append(*objects, condObj)
				}
			}
		}

		// ExitState sentinel
		*objects = append(*objects, &rive.ExitState{})
	}

	// --- Listeners ---
	for i := range sm.listeners {
		lc := &sm.listeners[i]
		obj := &rive.StateMachineListenerSingle{}
		obj.TargetId = lc.target.shapeIdx
		obj.ListenerTypeValue = uint64(lc.lt)
		obj.EventId = ^uint64(0) // suppress: pointer listener, not a Rive Event listener
		*objects = append(*objects, obj)

		// Emit action objects immediately after their parent listener.
		for _, ac := range lc.actions {
			switch ac.kind {
			case actionTrigger:
				a := &rive.ListenerTriggerChange{}
				a.InputId = ac.input.idx
				a.NestedInputId = ^uint64(0) // suppress: no nested artboard
				*objects = append(*objects, a)
			case actionSetBool:
				a := &rive.ListenerBoolChange{}
				a.InputId = ac.input.idx
				a.NestedInputId = ^uint64(0) // suppress: no nested artboard
				if ac.boolValue {
					a.Value = 1 // default=1, suppressed in binary
				}
				// Value=0 (false) is non-default, will be emitted as key=228
				*objects = append(*objects, a)
			}
		}
	}

	return nil
}

// newStateTransition constructs a StateTransition with the runtime-default
// values that the generated Properties() method would otherwise emit as zeros.
func newStateTransition(stateToId uint64) *rive.StateTransition {
	t := &rive.StateTransition{}
	t.StateToId = stateToId
	t.InterpolationType = 1       // runtime default (suppress emission)
	t.InterpolatorId = ^uint64(0) // sentinel (suppress emission)
	t.RandomWeight = 1            // runtime default (suppress emission)
	return t
}
