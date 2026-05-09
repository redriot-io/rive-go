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
		fillRelIdx := uint64(len(*objects)) - artboardOffset
		fill := &rive.Fill{}
		fill.ParentId = s.shapeIdx
		fill.IsVisible = true
		fill.BlendModeValue = 127
		*objects = append(*objects, fill)

		if s.fill.gradient != nil {
			s.emitGradient(objects, fillRelIdx, artboardOffset, s.fill.gradient)
		} else {
			s.solidColorIdx = uint64(len(*objects)) - artboardOffset
			s.hasSolidColorIdx = true
			sc := &rive.SolidColor{}
			sc.ColorValue = s.fill.color
			sc.ParentId = fillRelIdx
			*objects = append(*objects, sc)
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

func (s *ShapeRef) emitGradient(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64, g *gradientConfig) {
	gradRelIdx := uint64(len(*objects)) - artboardOffset
	var grad rive.Object
	if g.radial {
		rg := &rive.RadialGradient{}
		rg.StartX = g.x1
		rg.StartY = g.y1
		rg.EndX = g.x2
		rg.EndY = g.y2
		rg.ParentId = parentIdx
		rg.Opacity = 1.0
		grad = rg
	} else {
		lg := &rive.LinearGradient{}
		lg.StartX = g.x1
		lg.StartY = g.y1
		lg.EndX = g.x2
		lg.EndY = g.y2
		lg.ParentId = parentIdx
		lg.Opacity = 1.0 // runtime default; zero value would emit key#46=0 → invisible gradient
		grad = lg
	}
	*objects = append(*objects, grad)

	for _, stop := range g.stops {
		gs := &rive.GradientStop{}
		gs.ColorValue = stop.Color
		gs.Position = stop.Position
		gs.ParentId = gradRelIdx
		*objects = append(*objects, gs)
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
