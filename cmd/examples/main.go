// examples generates demo .riv files showcasing the rive-go builder API.
// Output is written to docs/preview/examples/.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/redriot-io/rive-go/rive/builder"
)

const outDir = "docs/preview/examples"

func main() {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	steps := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"fade_rect.riv", generateFadeRect},
		{"bounce_ball.riv", generateBounceBall},
		{"color_cycle.riv", generateColorCycle},
		{"toggle_button.riv", generateToggleButton},
		{"gradient_ellipse.riv", generateGradientEllipse},
		{"multi_shape.riv", generateMultiShape},
	}

	for _, s := range steps {
		data, err := s.fn()
		if err != nil {
			log.Fatalf("generate %s: %v", s.name, err)
		}
		path := filepath.Join(outDir, s.name)
		if err := os.WriteFile(path, data, 0644); err != nil {
			log.Fatalf("write %s: %v", path, err)
		}
		fmt.Printf("✓ %s (%d bytes)\n", path, len(data))
	}
	fmt.Println("Done.")
}

// 1. Red rectangle fades in over 30 frames, loops.
func generateFadeRect() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("FadeRect", 500, 500)

	rect := ab.Rectangle(150, 175, 200, 150).
		Fill(0xFFCC3333).
		Name("rect")

	ab.Animation("fadeIn",
		builder.WithDuration(30),
		builder.WithFPS(60),
		builder.WithLoop(builder.Loop),
	).
		KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropOpacity, 30, 1.0, builder.Linear())

	return b.Bytes()
}

// 2. Circle bounces up and down with cubic easing, ping-pong.
func generateBounceBall() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("BounceBall", 400, 400)

	ball := ab.Ellipse(200, 200, 60, 60).
		Fill(0xFF3399CC).
		Name("ball")

	ab.Animation("bounce",
		builder.WithDuration(60),
		builder.WithFPS(60),
		builder.WithLoop(builder.PingPong),
	).
		KeyframeFloat(ball, builder.PropY, 0, 300.0, builder.Cubic(0.42, 0, 0.58, 1)).
		KeyframeFloat(ball, builder.PropY, 60, 100.0, builder.Cubic(0.42, 0, 0.58, 1))

	return b.Bytes()
}

// 3. Rectangle animates through red→blue→green color keyframes, loops.
func generateColorCycle() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("ColorCycle", 400, 400)

	rect := ab.Rectangle(100, 100, 200, 200).
		Fill(0xFFFF0000). // start red (overridden by animation)
		Name("colorRect")

	ab.Animation("colorCycle",
		builder.WithDuration(90),
		builder.WithFPS(30),
		builder.WithLoop(builder.Loop),
	).
		KeyframeColor(rect, builder.PropColorValue, 0, 0xFFFF0000, builder.Linear()).  // red
		KeyframeColor(rect, builder.PropColorValue, 30, 0xFF0000FF, builder.Linear()). // blue
		KeyframeColor(rect, builder.PropColorValue, 60, 0xFF00FF00, builder.Linear()). // green
		KeyframeColor(rect, builder.PropColorValue, 90, 0xFFFF0000, builder.Linear())  // red (loop)

	return b.Bytes()
}

// 4. Toggle button: bool input "active" switches between Idle and Active states.
func generateToggleButton() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("ToggleButton", 300, 120)

	ab.Rectangle(20, 20, 260, 80).
		Fill(0xFF336699).
		Name("button")

	sm := ab.StateMachine("ButtonSM")
	active := sm.BoolInput("active")
	layer := sm.Layer("Main")
	idle := layer.State("Idle")
	on := layer.State("Active")
	layer.Transition(idle, on, builder.BoolCondition(active, true))
	layer.Transition(on, idle, builder.BoolCondition(active, false))

	return b.Bytes()
}

// 5. Ellipse with a linear gradient fill (static — tests shape+gradient rendering).
func generateGradientEllipse() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("GradientEllipse", 400, 400)

	ab.Ellipse(200, 200, 150, 150).
		FillGradient(50, 200, 350, 200,
			builder.GradientStop{Position: 0.0, Color: 0xFFFF6B6B},
			builder.GradientStop{Position: 0.5, Color: 0xFFFFD93D},
			builder.GradientStop{Position: 1.0, Color: 0xFF6BCB77},
		).
		Name("gradEllipse")

	return b.Bytes()
}

// 6. Multiple shapes: rect + ellipse + stroked rect in one artboard.
func generateMultiShape() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("MultiShape", 600, 400)

	ab.Rectangle(50, 100, 150, 200).
		Fill(0xFFE74C3C).
		Name("redRect")

	ab.Ellipse(300, 200, 90, 90).
		Fill(0xFF3498DB).
		Name("blueCircle")

	ab.Rectangle(420, 100, 140, 200).
		Fill(0xFF2ECC71).
		Stroke(4.0, 0xFF27AE60).
		Name("greenRect")

	return b.Bytes()
}
