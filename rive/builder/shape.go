package builder

import (
	"math"

	"github.com/redriot-io/rive-go/rive"
)

type shapeKind uint8

const (
	shapeRect    shapeKind = iota
	shapeEllipse shapeKind = iota
)

// GradientStop defines a color stop in a linear gradient.
type GradientStop struct {
	Position float64 // 0.0 to 1.0
	Color    uint32  // ARGB packed
}

type fillConfig struct {
	color    uint32
	gradient *gradientConfig
}

type gradientConfig struct {
	x1, y1, x2, y2 float64
	stops           []GradientStop
	radial          bool
}

type strokeConfig struct {
	thickness float64
	color     uint32
}

// ShapeRef is a handle to a shape added to an artboard. It is used for paint
// configuration and as an animation target.
type ShapeRef struct {
	kind         shapeKind
	name         string
	x, y         float64
	param1, param2 float64 // width/height or radiusX/radiusY

	opacitySet       bool
	opacity          float64
	rotationSet      bool
	rotationDeg      float64 // stored in degrees, converted to radians on emit
	scaleSet         bool
	scaleX           float64
	scaleY           float64
	cornerRadiusSet  bool
	cornerRadius     float64

	fill   *fillConfig
	stroke *strokeConfig

	drawRules []drawRuleConfig
	clips     []*PathRef
	skins     []*SkinBinding

	// Set during emitObjects — used by AnimationBuilder to resolve objectIds.
	shapeIdx uint64 // Shape object global index
	pathIdx  uint64 // Rectangle/Ellipse global index
	// solidColorIdx: global index of the first SolidColor (for fill color animation)
	solidColorIdx    uint64
	hasSolidColorIdx bool
}

func newShapeRef(kind shapeKind, x, y, p1, p2 float64) *ShapeRef {
	return &ShapeRef{kind: kind, x: x, y: y, param1: p1, param2: p2}
}

// Fill sets a solid color fill on this shape.
func (s *ShapeRef) Fill(color uint32) *ShapeRef {
	s.fill = &fillConfig{color: color}
	return s
}

// FillGradient sets a linear gradient fill.
func (s *ShapeRef) FillGradient(x1, y1, x2, y2 float64, stops ...GradientStop) *ShapeRef {
	s.fill = &fillConfig{gradient: &gradientConfig{x1: x1, y1: y1, x2: x2, y2: y2, stops: stops}}
	return s
}

// FillRadialGradient sets a radial gradient fill.
// cx, cy is the center in shape-local space; ex, ey is a point on the edge
// (distance from center to edge = radius).
func (s *ShapeRef) FillRadialGradient(cx, cy, ex, ey float64, stops ...GradientStop) *ShapeRef {
	s.fill = &fillConfig{gradient: &gradientConfig{x1: cx, y1: cy, x2: ex, y2: ey, stops: stops, radial: true}}
	return s
}

// Stroke sets a stroke on this shape.
func (s *ShapeRef) Stroke(width float64, color uint32) *ShapeRef {
	s.stroke = &strokeConfig{thickness: width, color: color}
	return s
}

// Opacity sets the shape's opacity (0.0 – 1.0).
func (s *ShapeRef) Opacity(v float64) *ShapeRef {
	s.opacitySet = true
	s.opacity = v
	return s
}

// Rotation sets the shape's initial rotation in degrees (clockwise).
// The value is converted to radians when emitting the Rive binary.
func (s *ShapeRef) Rotation(degrees float64) *ShapeRef {
	s.rotationSet = true
	s.rotationDeg = degrees
	return s
}

// Scale sets the shape's initial X and Y scale factors.
func (s *ShapeRef) Scale(sx, sy float64) *ShapeRef {
	s.scaleSet = true
	s.scaleX = sx
	s.scaleY = sy
	return s
}

// CornerRadius sets the corner radius for rectangle shapes.
func (s *ShapeRef) CornerRadius(r float64) *ShapeRef {
	s.cornerRadiusSet = true
	s.cornerRadius = r
	return s
}

// Name sets a name for the shape (useful for animation targeting and debugging).
func (s *ShapeRef) Name(n string) *ShapeRef {
	s.name = n
	return s
}

// DrawAbove makes this shape render in front of (above) target in the draw order.
// Under the hood it creates a DrawRules object (typeKey=49) as a child of this shape
// and a DrawTarget object (typeKey=48) as a child of the DrawRules, referencing
// target with PlacementAbove. Both objects are emitted in a post-pass after all
// shapes so that every artboard-relative index is resolved.
// Returns s for chaining.
func (s *ShapeRef) DrawAbove(target *ShapeRef) *ShapeRef {
	s.drawRules = append(s.drawRules, drawRuleConfig{target: target, placement: PlacementAbove})
	return s
}

// DrawBelow makes this shape render behind (below) target in the draw order.
// Creates the same DrawRules+DrawTarget pair as DrawAbove but with PlacementBelow.
// Returns s for chaining.
func (s *ShapeRef) DrawBelow(target *ShapeRef) *ShapeRef {
	s.drawRules = append(s.drawRules, drawRuleConfig{target: target, placement: PlacementBelow})
	return s
}

// emitDrawRules emits DrawRules + DrawTarget object pairs for each draw rule.
// DrawRules is emitted first; DrawTarget follows as its child (ParentId = DrawRules index).
// Must be called after emitObjects() so that shapeIdx values are resolved.
func (s *ShapeRef) emitDrawRules(objects *[]rive.Object, artboardOffset uint64) {
	for _, rule := range s.drawRules {
		// DrawRules is emitted first so DrawTarget can reference it as parent.
		drIdx := uint64(len(*objects)) - artboardOffset
		dtIdx := drIdx + 1 // DrawTarget follows immediately

		dr := &rive.DrawRules{}
		dr.ParentId = s.shapeIdx
		dr.DrawTargetId = dtIdx
		*objects = append(*objects, dr)

		dt := &rive.DrawTarget{}
		dt.ParentId = drIdx // child of DrawRules, matching Rive editor output
		dt.DrawableId = rule.target.shapeIdx
		dt.PlacementValue = rule.placement
		*objects = append(*objects, dt)
	}
}

// emitObjects writes the Shape, its path child, and its paint children
// into the object list, recording indices for later animation use.
func (s *ShapeRef) emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	// --- Shape (transform node) ---
	s.shapeIdx = uint64(len(*objects)) - artboardOffset
	shape := &rive.Shape{}
	shape.Name = s.name
	shape.ParentId = parentIdx
	shape.X = s.x
	shape.Y = s.y
	shape.Opacity = 1.0
	shape.ScaleX = 1.0
	shape.ScaleY = 1.0
	shape.BlendModeValue = 3
	if s.opacitySet {
		shape.Opacity = s.opacity
	}
	if s.rotationSet {
		shape.Rotation = s.rotationDeg * math.Pi / 180.0
	}
	if s.scaleSet {
		shape.ScaleX = s.scaleX
		shape.ScaleY = s.scaleY
	}
	*objects = append(*objects, shape)

	// --- Path child (Rectangle or Ellipse) ---
	s.pathIdx = uint64(len(*objects)) - artboardOffset
	switch s.kind {
	case shapeRect:
		r := &rive.Rectangle{}
		r.Width = s.param1
		r.Height = s.param2
		r.ParentId = s.shapeIdx
		r.Opacity = 1.0
		r.ScaleX = 1.0
		r.ScaleY = 1.0
		r.OriginX = 0.5
		r.OriginY = 0.5
		r.LinkCornerRadius = true
		if s.cornerRadiusSet {
			r.CornerRadiusTL = s.cornerRadius
		}
		*objects = append(*objects, r)
	case shapeEllipse:
		e := &rive.Ellipse{}
		e.Width = s.param1
		e.Height = s.param2
		e.ParentId = s.shapeIdx
		e.Opacity = 1.0
		e.ScaleX = 1.0
		e.ScaleY = 1.0
		e.OriginX = 0.5
		e.OriginY = 0.5
		*objects = append(*objects, e)
	}

	// --- Fill paint ---
	if s.fill != nil {
		if s.fill.gradient != nil {
			fillRelIdx := uint64(len(*objects)) - artboardOffset
			fill := &rive.Fill{}
			fill.ParentId = s.shapeIdx
			fill.IsVisible = true
			fill.BlendModeValue = 127
			*objects = append(*objects, fill)
			emitGradient(objects, fillRelIdx, artboardOffset, s.fill.gradient)
		} else {
			// SolidColor emitted BEFORE Fill (official encoder forward-reference pattern).
			// SolidColor.parentId points forward to Fill (the very next slot).
			s.solidColorIdx = uint64(len(*objects)) - artboardOffset
			s.hasSolidColorIdx = true
			fillFwdRef := s.solidColorIdx + 1
			sc := &rive.SolidColor{}
			sc.ColorValue = s.fill.color
			sc.ParentId = fillFwdRef
			*objects = append(*objects, sc)

			fill := &rive.Fill{}
			fill.ParentId = s.shapeIdx
			fill.IsVisible = true
			fill.BlendModeValue = 127
			*objects = append(*objects, fill)
		}
	}

	// --- Stroke paint ---
	if s.stroke != nil {
		strokeRelIdx := uint64(len(*objects)) - artboardOffset
		st := &rive.Stroke{}
		st.Thickness = s.stroke.thickness
		st.ParentId = s.shapeIdx
		st.IsVisible = true
		st.BlendModeValue = 127
		*objects = append(*objects, st)

		sc := &rive.SolidColor{}
		sc.ColorValue = s.stroke.color
		sc.ParentId = strokeRelIdx
		*objects = append(*objects, sc)
	}
}

// animIdx returns the artboard-relative index of this shape's Shape object.
func (s *ShapeRef) animIdx() uint64 { return s.shapeIdx }

// animColorIdx returns the artboard-relative index of this shape's SolidColor object.
func (s *ShapeRef) animColorIdx() (uint64, bool) { return s.solidColorIdx, s.hasSolidColorIdx }

// ClipWith adds a clipping mask to this shape using the geometry of p.
// The clip is applied after all shapes are emitted (post-pass).
func (s *ShapeRef) ClipWith(p *PathRef) *ShapeRef {
	s.clips = append(s.clips, p)
	return s
}

// emitClips emits a ClippingShape child for each clip source added via ClipWith.
// Must be called after emitObjects so that shapeIdx and pathIdx are resolved.
func (s *ShapeRef) emitClips(objects *[]rive.Object, artboardOffset uint64) {
	for _, p := range s.clips {
		cs := &rive.ClippingShape{}
		cs.ParentId = s.shapeIdx
		cs.SourceId = p.pathIdx
		cs.IsVisible = true
		*objects = append(*objects, cs)
	}
}

// NodeRef is a handle to a plain transform node added to an artboard.
type NodeRef struct {
	name string
	x, y float64
	idx  uint64
}

func (n *NodeRef) emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	n.idx = uint64(len(*objects)) - artboardOffset
	node := &rive.Node{}
	node.Name = n.name
	node.X = n.x
	node.Y = n.y
	node.ParentId = parentIdx
	node.Opacity = 1.0
	node.ScaleX = 1.0
	node.ScaleY = 1.0
	*objects = append(*objects, node)
}
