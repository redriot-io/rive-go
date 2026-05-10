package builder

// ListenerType identifies which pointer event a StateMachineListenerSingle fires on.
type ListenerType uint64

const (
	ListenerPointerDown  ListenerType = 0
	ListenerPointerUp    ListenerType = 1
	ListenerPointerEnter ListenerType = 2
	ListenerPointerExit  ListenerType = 3
	ListenerPointerMove  ListenerType = 4
	ListenerClick        ListenerType = 5
)

type actionKind uint8

const (
	actionTrigger actionKind = iota
	actionSetBool
)

type actionConfig struct {
	kind      actionKind
	input     *InputRef
	boolValue bool
}

// listenerConfig holds the data for a single listener on a state machine.
type listenerConfig struct {
	target  *ShapeRef
	lt      ListenerType
	actions []actionConfig
}

// ListenerRef is a handle to a listener. Use SetTrigger / SetBool to bind
// pointer events to state machine inputs.
type ListenerRef struct {
	cfg *listenerConfig
}

// SetTrigger adds an action that fires input when this listener's event occurs.
func (l *ListenerRef) SetTrigger(input *InputRef) *ListenerRef {
	l.cfg.actions = append(l.cfg.actions, actionConfig{kind: actionTrigger, input: input})
	return l
}

// SetBool adds an action that sets input to value when this listener's event occurs.
func (l *ListenerRef) SetBool(input *InputRef, value bool) *ListenerRef {
	l.cfg.actions = append(l.cfg.actions, actionConfig{kind: actionSetBool, input: input, boolValue: value})
	return l
}
