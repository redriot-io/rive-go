// Package builder provides a fluent API for constructing Rive scenes
// programmatically. It produces an ordered []rive.Object slice ready for
// rive.WriteBytes().
//
// Basic usage:
//
//	b := builder.New()
//	ab := b.Artboard("Main", 500, 500)
//	rect := ab.Rectangle(100, 100, 200, 150).Fill(0xFFFF0000).Name("myRect")
//	ab.Animation("fadeIn", builder.WithDuration(30)).
//	    KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Linear()).
//	    KeyframeFloat(rect, builder.PropOpacity, 30, 1.0, builder.Linear())
//	data, err := b.Bytes()
package builder

import (
	"errors"

	"github.com/redriot-io/rive-go/rive"
)

// Builder constructs a complete .riv file's object graph.
type Builder struct {
	artboards []*ArtboardBuilder
}

// New creates a new scene builder.
func New() *Builder {
	return &Builder{}
}

// Artboard adds an artboard to the scene and returns its builder.
func (b *Builder) Artboard(name string, width, height float64) *ArtboardBuilder {
	ab := &ArtboardBuilder{name: name, width: width, height: height}
	b.artboards = append(b.artboards, ab)
	return ab
}

// Build finalizes and returns the ordered object slice.
// Automatically prepends Backboard. Returns an error if no artboards are defined.
func (b *Builder) Build() ([]rive.Object, error) {
	if len(b.artboards) == 0 {
		return nil, errors.New("builder: at least one artboard is required")
	}
	var objects []rive.Object
	// index 0: Backboard
	objects = append(objects, &rive.Backboard{})
	for _, ab := range b.artboards {
		if err := ab.emit(&objects); err != nil {
			return nil, err
		}
	}
	return objects, nil
}

// Bytes is a convenience wrapper: Build() then rive.WriteBytes().
func (b *Builder) Bytes(opts ...rive.WriteOption) ([]byte, error) {
	objects, err := b.Build()
	if err != nil {
		return nil, err
	}
	return rive.WriteBytes(objects, opts...)
}

// ArtboardBuilder builds a single artboard and all of its contents.
type ArtboardBuilder struct {
	name          string
	width, height float64

	children      []childEmitter
	animations    []*AnimationBuilder
	stateMachines []*StateMachineBuilder
}

// childEmitter is implemented by shapes, paths, and nodes that can be added
// directly to an artboard or node.
type childEmitter interface {
	emitObjects(objects *[]rive.Object, parentIdx uint64)
}

// Rectangle adds a rectangle child to the artboard.
func (ab *ArtboardBuilder) Rectangle(x, y, width, height float64) *ShapeRef {
	sr := newShapeRef(shapeRect, x, y, width, height)
	ab.children = append(ab.children, sr)
	return sr
}

// Ellipse adds an ellipse child to the artboard. param1/param2 are radiusX/radiusY,
// stored internally as the ParametricPath width/height.
func (ab *ArtboardBuilder) Ellipse(x, y, radiusX, radiusY float64) *ShapeRef {
	sr := newShapeRef(shapeEllipse, x, y, radiusX, radiusY)
	ab.children = append(ab.children, sr)
	return sr
}

// Node adds a group/transform node to the artboard.
func (ab *ArtboardBuilder) Node(name string, x, y float64) *NodeRef {
	nr := &NodeRef{name: name, x: x, y: y}
	ab.children = append(ab.children, nr)
	return nr
}

// Animation adds a linear animation to this artboard and returns its builder.
func (ab *ArtboardBuilder) Animation(name string, opts ...AnimationOption) *AnimationBuilder {
	a := newAnimationBuilder(name, opts...)
	ab.animations = append(ab.animations, a)
	return a
}

// StateMachine adds a state machine to this artboard and returns its builder.
func (ab *ArtboardBuilder) StateMachine(name string) *StateMachineBuilder {
	sm := &StateMachineBuilder{name: name}
	ab.stateMachines = append(ab.stateMachines, sm)
	return sm
}

func (ab *ArtboardBuilder) emit(objects *[]rive.Object) error {
	artboardIdx := uint64(len(*objects))

	a := &rive.Artboard{}
	a.Name = ab.name
	a.Width = ab.width
	a.Height = ab.height
	a.ParentId = 0
	// Runtime defaults — Go zero-values differ from Rive runtime defaults
	a.Opacity = 1.0
	a.ScaleX = 1.0
	a.ScaleY = 1.0
	a.BlendModeValue = 3
	a.FractionalWidth = 1.0
	a.FractionalHeight = 1.0
	a.StyleId = ^uint64(0)
	a.DefaultStateMachineId = ^uint64(0)
	a.ViewModelId = ^uint64(0)
	*objects = append(*objects, a)

	// Emit children (shapes, nodes) depth-first
	for _, child := range ab.children {
		child.emitObjects(objects, artboardIdx)
	}

	// Emit animations (after all artboard children, so ShapeRef.idx are set)
	for _, anim := range ab.animations {
		if err := anim.emit(objects); err != nil {
			return err
		}
	}

	// Emit state machines (after animations, so AnimationBuilder.idx are set)
	for _, sm := range ab.stateMachines {
		if err := sm.emit(objects, ab.animations); err != nil {
			return err
		}
	}

	return nil
}
