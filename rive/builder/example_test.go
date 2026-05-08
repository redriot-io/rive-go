package builder_test

import (
	"fmt"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// Example_basicScene demonstrates building a minimal .riv scene with a
// rectangle that fades in over 30 frames.
func Example_basicScene() {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)

	rect := ab.Rectangle(100, 100, 200, 150).
		Fill(0xFFFF0000). // red fill
		Name("myRect")

	ab.Animation("fadeIn", builder.WithDuration(30), builder.WithFPS(60)).
		KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropOpacity, 30, 1.0, builder.Linear())

	data, err := b.Bytes()
	if err != nil {
		panic(err)
	}

	f, _ := rive.ReadBytes(data)
	fmt.Printf("objects: %d\n", len(f.Objects))
	// Output: objects: 11
}

// Example_stateMachine demonstrates a simple two-state toggle driven by a
// bool input.
func Example_stateMachine() {
	b := builder.New()
	ab := b.Artboard("Button", 200, 80)
	ab.Rectangle(0, 0, 200, 80).Fill(0xFF336699)

	sm := ab.StateMachine("ButtonSM")
	toggle := sm.BoolInput("isActive")
	layer := sm.Layer("Main")
	off := layer.State("Off")
	on := layer.State("On")
	layer.Transition(off, on, builder.BoolCondition(toggle, true))
	layer.Transition(on, off, builder.BoolCondition(toggle, false))

	data, err := b.Bytes()
	if err != nil {
		panic(err)
	}

	f, _ := rive.ReadBytes(data)
	fmt.Printf("ok, objects: %d\n", len(f.Objects))
	// Output: ok, objects: 19
}
