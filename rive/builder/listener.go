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

// listenerConfig holds the data for a single listener on a state machine.
type listenerConfig struct {
	target *ShapeRef
	lt     ListenerType
}

// ListenerRef is a handle to a listener, reserved for future action binding.
type ListenerRef struct {
	cfg *listenerConfig
}
