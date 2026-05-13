package main

import (
	"fmt"

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

// minimalPNG is a 1×1 opaque red PNG for Image type testing.
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
