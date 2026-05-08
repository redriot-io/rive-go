package main

import "github.com/redriot-io/rive-go/rive/builder"

// generateMinimalStatic creates the simplest possible renderable .riv:
// 1 Artboard (500×500) + 1 Rectangle (centered 100×100) + 1 solid-red Fill.
// No animation, no state machine. Used to isolate format issues from animation bugs.
func generateMinimalStatic() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Test", 500, 500)
	ab.Rectangle(200, 200, 100, 100).
		Fill(0xFFFF0000).
		Name("rect")
	return b.Bytes()
}
