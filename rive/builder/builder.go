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

	// Emit all font assets globally BEFORE any Artboard object. The official Rive
	// encoder always places FontAsset/FileAssetContents pairs between Backboard and
	// the first Artboard. FontRef.idx is the 0-based index in this font asset list
	// (not artboard-relative), which is what fontAssetId (key 279) on TextStyle uses.
	fontListIdx := uint64(0)
	for _, ab := range b.artboards {
		for _, f := range ab.fonts {
			f.idx = fontListIdx
			fontListIdx++
			fa := &rive.FontAsset{}
			fa.Name = f.name
			objects = append(objects, fa)
			fac := &rive.FileAssetContents{}
			fac.Bytes = f.ttfBytes
			objects = append(objects, fac)
		}
	}

	// Emit all image assets globally BEFORE any Artboard, after fonts.
	// ImageRef.idx is 0-based index among ImageAssets in the global stream.
	imageListIdx := uint64(0)
	for _, ab := range b.artboards {
		for _, img := range ab.images {
			img.idx = imageListIdx
			imageListIdx++
			ia := &rive.ImageAsset{}
			ia.Name = img.name
			objects = append(objects, ia)
			fac2 := &rive.FileAssetContents{}
			fac2.Bytes = img.pngBytes
			objects = append(objects, fac2)
		}
	}

	// Emit all audio assets globally BEFORE any Artboard, after images.
	// AudioAssetRef.idx is 0-based index among AudioAssets in the global stream.
	audioListIdx := uint64(0)
	for _, ab := range b.artboards {
		for _, a := range ab.audios {
			a.idx = audioListIdx
			audioListIdx++
			aa := &rive.AudioAsset{}
			aa.Name = a.name
			aa.Volume = a.volume
			objects = append(objects, aa)
			fac3 := &rive.FileAssetContents{}
			fac3.Bytes = a.audioBytes
			objects = append(objects, fac3)
		}
	}

	for _, ab := range b.artboards {
		if err := ab.emit(&objects); err != nil {
			return nil, err
		}
	}

	// Phase 2: Reorder per format contract (SolidColor before Fill).
	objects = rive.ReorderByContract(objects)
	// Phase 3: Recompute parentIds after reorder (idempotent; ReorderByContract
	// already fixes them, but called explicitly per spec for clarity).
	rive.FixParentIds(objects)

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

	fonts         []*FontRef
	images        []*ImageRef
	audios        []*AudioAssetRef
	boneTrees     []*BoneRef
	children      []childEmitter
	animations    []*AnimationBuilder
	stateMachines []*StateMachineBuilder
}

// childEmitter is implemented by shapes, paths, and nodes that can be added
// directly to an artboard or node.
type childEmitter interface {
	emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64)
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

// EmbedFont registers a font asset with raw TTF/OTF bytes and returns a FontRef.
// The FontRef is passed to TextRef.Style to associate a font with a text style.
// Fonts are emitted before text objects in the artboard's object stream.
func (ab *ArtboardBuilder) EmbedFont(name string, ttfBytes []byte) *FontRef {
	f := &FontRef{name: name, ttfBytes: ttfBytes}
	ab.fonts = append(ab.fonts, f)
	return f
}

// Text adds a text object to the artboard and returns its builder.
// Set position and style with the returned TextRef's fluent methods.
func (ab *ArtboardBuilder) Text(name string) *TextRef {
	t := &TextRef{name: name}
	ab.children = append(ab.children, t)
	return t
}

// Path adds a custom path shape to the artboard. Vertices are added with
// .LineTo / .CubicTo; call .Close() to close the path before encoding.
func (ab *ArtboardBuilder) Path(x, y float64) *PathRef {
	pr := &PathRef{x: x, y: y}
	ab.children = append(ab.children, pr)
	return pr
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
	artboardOffset := uint64(len(*objects))

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

	// Emit bone trees before other children (bones must precede shapes for Tendon.BoneId).
	for _, bt := range ab.boneTrees {
		emitBoneTree(bt, objects, artboardOffset)
	}

	// Emit children in REVERSE declaration order.
	// The Rive runtime renders first-emitted children IN FRONT (not painter's algorithm).
	// Reversing here means JSON children[0] (declared first) emits last → renders at the back.
	for i := len(ab.children) - 1; i >= 0; i-- {
		ab.children[i].emitObjects(objects, 0, artboardOffset)
	}

	// Emit animations (after all artboard children, so ShapeRef.idx are set)
	for _, anim := range ab.animations {
		if err := anim.emit(objects, artboardOffset); err != nil {
			return err
		}
	}

	// Emit state machines (after animations, so AnimationBuilder.idx are set)
	for _, sm := range ab.stateMachines {
		if err := sm.emit(objects, ab.animations); err != nil {
			return err
		}
	}

	// Emit skin bindings post-pass: bone indices are resolved, shapeIdx is set.
	for _, child := range ab.children {
		if sr, ok := child.(*ShapeRef); ok {
			sr.emitSkins(objects, artboardOffset)
		}
	}

	// Emit draw rules post-pass: all shapeIdx/pathIdx values are resolved at this point.
	for _, child := range ab.children {
		switch c := child.(type) {
		case *ShapeRef:
			c.emitDrawRules(objects, artboardOffset)
		case *PathRef:
			c.emitDrawRules(objects, artboardOffset)
		}
	}

	// Emit clipping shapes post-pass: SourceId resolved after all children emitted.
	for _, child := range ab.children {
		if sr, ok := child.(*ShapeRef); ok {
			sr.emitClips(objects, artboardOffset)
		}
	}

	return nil
}
