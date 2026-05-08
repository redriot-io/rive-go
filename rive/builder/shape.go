package builder

import "github.com/redriot-io/rive-go/rive"

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

	opacitySet bool
	opacity    float64

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

// Name sets a name for the shape (useful for animation targeting and debugging).
func (s *ShapeRef) Name(n string) *ShapeRef {
	s.name = n
	return s
}

// emitObjects writes the Shape, its path child, and its paint children
// into the object list, recording indices for later animation use.
func (s *ShapeRef) emitObjects(objects *[]rive.Object, parentIdx uint64) {
	// --- Shape (transform node) ---
	s.shapeIdx = uint64(len(*objects))
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
	*objects = append(*objects, shape)

	// --- Path child (Rectangle or Ellipse) ---
	s.pathIdx = uint64(len(*objects))
	switch s.kind {
	case shapeRect:
		r := &rive.Rectangle{}
		r.Width = s.param1
		r.Height = s.param2
		r.ParentId = s.shapeIdx
		r.Opacity = 1.0
		r.ScaleX = 1.0
		r.ScaleY = 1.0
		r.LinkCornerRadius = true
		*objects = append(*objects, r)
	case shapeEllipse:
		e := &rive.Ellipse{}
		e.Width = s.param1
		e.Height = s.param2
		e.ParentId = s.shapeIdx
		e.Opacity = 1.0
		e.ScaleX = 1.0
		e.ScaleY = 1.0
		*objects = append(*objects, e)
	}

	// --- Fill paint ---
	if s.fill != nil {
		fillIdx := uint64(len(*objects))
		fill := &rive.Fill{}
		fill.ParentId = s.shapeIdx
		*objects = append(*objects, fill)

		if s.fill.gradient != nil {
			s.emitGradient(objects, fillIdx, s.fill.gradient)
		} else {
			s.solidColorIdx = uint64(len(*objects))
			s.hasSolidColorIdx = true
			sc := &rive.SolidColor{}
			sc.ColorValue = s.fill.color
			sc.ParentId = fillIdx
			*objects = append(*objects, sc)
		}
	}

	// --- Stroke paint ---
	if s.stroke != nil {
		strokeIdx := uint64(len(*objects))
		st := &rive.Stroke{}
		st.Thickness = s.stroke.thickness
		st.ParentId = s.shapeIdx
		*objects = append(*objects, st)

		sc := &rive.SolidColor{}
		sc.ColorValue = s.stroke.color
		sc.ParentId = strokeIdx
		*objects = append(*objects, sc)
	}
}

func (s *ShapeRef) emitGradient(objects *[]rive.Object, parentIdx uint64, g *gradientConfig) {
	gradIdx := uint64(len(*objects))
	lg := &rive.LinearGradient{}
	lg.StartX = g.x1
	lg.StartY = g.y1
	lg.EndX = g.x2
	lg.EndY = g.y2
	lg.ParentId = parentIdx
	*objects = append(*objects, lg)

	for _, stop := range g.stops {
		gs := &rive.GradientStop{}
		gs.ColorValue = stop.Color
		gs.Position = stop.Position
		gs.ParentId = gradIdx
		*objects = append(*objects, gs)
	}
}

// NodeRef is a handle to a plain transform node added to an artboard.
type NodeRef struct {
	name string
	x, y float64
	idx  uint64
}

func (n *NodeRef) emitObjects(objects *[]rive.Object, parentIdx uint64) {
	n.idx = uint64(len(*objects))
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
