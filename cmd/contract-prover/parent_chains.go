package main

import (
	"fmt"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// parentChains documents the required object ancestry for each testable type.
// The prover uses this for topological ordering (dependencies before dependents)
// and for documentation in format_contract_proven.json.
var parentChains = map[string][]string{
	"Artboard":        {},
	"Node":            {"Artboard"},
	"Shape":           {"Artboard"},
	"Rectangle":       {"Artboard", "Shape"},
	"Ellipse":         {"Artboard", "Shape"},
	"ParametricPath":  {"Artboard", "Shape"},
	"Fill":            {"Artboard", "Shape"},
	"Stroke":          {"Artboard", "Shape"},
	"SolidColor":      {"Artboard", "Shape", "Fill"},
	"LinearGradient":  {"Artboard", "Shape", "Fill"},
	"LinearAnimation": {"Artboard"},
	"KeyedObject":     {"Artboard", "LinearAnimation"},
	"KeyedProperty":   {"Artboard", "LinearAnimation", "KeyedObject"},
	"Image":           {"Artboard"},
	"StateMachine":    {"Artboard"},
}

// typeOrder defines the processing order: dependencies before dependents.
// This is a hand-maintained topological sort of parentChains.
var typeOrder = []string{
	"Artboard",
	"Node",
	"Shape",
	"Rectangle",
	"Ellipse",
	"ParametricPath",
	"Fill",
	"Stroke",
	"SolidColor",
	"LinearGradient",
	"LinearAnimation",
	"KeyedObject",
	"KeyedProperty",
	"Image",
	"StateMachine",
}

// buildFuncs maps each type name to a function that produces minimal valid .riv bytes.
// Each function constructs the minimal scene (Backboard + Artboard + parent chain + type)
// using the builder API, which already encodes all runtime-required defaults.
var buildFuncs = map[string]func() ([]byte, error){
	"Artboard":        buildArtboard,
	"Node":            buildNode,
	"Shape":           buildShape,
	"Rectangle":       buildRectangle,
	"Ellipse":         buildEllipse,
	"ParametricPath":  buildParametricPath,
	"Fill":            buildFill,
	"Stroke":          buildStroke,
	"SolidColor":      buildSolidColor,
	"LinearGradient":  buildLinearGradient,
	"LinearAnimation": buildLinearAnimation,
	"KeyedObject":     buildKeyedObject,
	"KeyedProperty":   buildKeyedProperty,
	"Image":           buildImage,
	"StateMachine":    buildStateMachine,
}

// bisectFuncs maps type names to BisectFunc implementations for bisection testing.
// These build a deliberately broken .riv (required defaults missing/zeroed) and apply
// only the specified candidates. Used with --force-fail to demonstrate bisection.
var bisectFuncs = map[string]BisectFunc{
	"Image": bisectImageFunc,
}

// ── Normal build functions ───────────────────────────────────────────────────

func buildArtboard() ([]byte, error) {
	b := builder.New()
	b.Artboard("Main", 100, 100)
	return b.Bytes()
}

func buildNode() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Node("node0", 50, 50)
	return b.Bytes()
}

func buildShape() ([]byte, error) {
	// Shape is always accompanied by a ParametricPath child in the builder.
	// A Rectangle with no paint is the minimal scene containing a Shape.
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Rectangle(10, 10, 80, 80)
	return b.Bytes()
}

func buildRectangle() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Rectangle(10, 10, 80, 80).Fill(0xFFFF0000)
	return b.Bytes()
}

func buildEllipse() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Ellipse(50, 50, 40, 40).Fill(0xFF00FF00)
	return b.Bytes()
}

func buildParametricPath() ([]byte, error) {
	// ParametricPath is the abstract base for Rectangle and Ellipse.
	// Exercised via Rectangle.
	return buildRectangle()
}

func buildFill() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Rectangle(10, 10, 80, 80).Fill(0xFFFF0000)
	return b.Bytes()
}

func buildStroke() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Rectangle(10, 10, 80, 80).Stroke(2.0, 0xFF0000FF)
	return b.Bytes()
}

func buildSolidColor() ([]byte, error) {
	// SolidColor is always paired with a Fill or Stroke.
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Rectangle(10, 10, 80, 80).Fill(0xFFFF0000)
	return b.Bytes()
}

func buildLinearGradient() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.Rectangle(10, 10, 80, 80).FillGradient(
		0, 0, 80, 0,
		builder.GradientStop{Position: 0.0, Color: 0xFFFF0000},
		builder.GradientStop{Position: 1.0, Color: 0xFF0000FF},
	)
	return b.Bytes()
}

func buildLinearAnimation() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	rect := ab.Rectangle(10, 10, 80, 80).Fill(0xFFFF0000)
	ab.Animation("anim").
		KeyframeFloat(rect, builder.PropX, 0, 10, builder.Linear()).
		KeyframeFloat(rect, builder.PropX, 60, 90, builder.Linear())
	return b.Bytes()
}

func buildKeyedObject() ([]byte, error) {
	// KeyedObject is created implicitly when an animation targets an object.
	return buildLinearAnimation()
}

func buildKeyedProperty() ([]byte, error) {
	// KeyedProperty is created implicitly when an animation targets a property.
	return buildLinearAnimation()
}

// minimalPNG is a 1×1 transparent PNG used for Image type testing.
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

func buildImage() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	asset := ab.EmbedImage("testimg", minimalPNG)
	ab.Image(asset).Position(50, 50)
	return b.Bytes()
}

func buildStateMachine() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	ab.StateMachine("sm")
	return b.Bytes()
}

// ── Bisect build functions ───────────────────────────────────────────────────

// bisectImageFunc builds a broken Image scene for bisection testing.
// The broken state zeros BlendModeValue, Opacity, ScaleX, ScaleY on the Image node.
// Only the specified candidates are applied back before serialization.
// Calling with nil/empty candidates produces the pure broken state (expected to FAIL WASM).
func bisectImageFunc(candidates []CandidateProp) ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Main", 100, 100)
	asset := ab.EmbedImage("testimg", minimalPNG)
	ab.Image(asset).Position(50, 50)

	objects, err := b.Build()
	if err != nil {
		return nil, err
	}

	// Find the Image node and corrupt its required defaults.
	for _, obj := range objects {
		img, ok := obj.(*rive.Image)
		if !ok {
			continue
		}
		// Zero out all required defaults (simulates missing defaults).
		img.BlendModeValue = 0 // Go zero; guard !=3 will emit key 23=0 → WASM null load
		img.Opacity = 0        // guard !=1 will emit key 18=0
		img.ScaleX = 0         // guard !=1 will emit key 16=0
		img.ScaleY = 0         // guard !=1 will emit key 17=0

		// Apply only the specified candidate properties.
		for _, c := range candidates {
			applyImageCandidate(img, c)
		}
		break
	}

	return rive.WriteBytes(objects)
}

// applyImageCandidate applies a single candidate property to an Image object.
func applyImageCandidate(img *rive.Image, c CandidateProp) {
	switch c.Name {
	case "blendModeValue":
		img.BlendModeValue = toUint64(c.Value)
	case "opacity":
		img.Opacity = toFloat64(c.Value)
	case "scaleX":
		img.ScaleX = toFloat64(c.Value)
	case "scaleY":
		img.ScaleY = toFloat64(c.Value)
	}
}

// ── Type coercion helpers ────────────────────────────────────────────────────

// toUint64 converts JSON-decoded interface{} to uint64.
// JSON numbers decode as float64 in Go.
func toUint64(v interface{}) uint64 {
	switch x := v.(type) {
	case float64:
		return uint64(x)
	case uint64:
		return x
	case int:
		return uint64(x)
	}
	return 0
}

// toFloat64 converts JSON-decoded interface{} to float64.
func toFloat64(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case uint64:
		return float64(x)
	}
	return 0
}

// ── Init validation ──────────────────────────────────────────────────────────

func init() {
	for _, t := range typeOrder {
		if _, ok := buildFuncs[t]; !ok {
			panic(fmt.Sprintf("contract-prover: no build function for type %q in buildFuncs", t))
		}
		if _, ok := parentChains[t]; !ok {
			panic(fmt.Sprintf("contract-prover: no parent chain for type %q in parentChains", t))
		}
	}
}
