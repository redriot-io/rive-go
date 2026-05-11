package builder

import (
	"math"

	"github.com/redriot-io/rive-go/rive"
)

// PathVertexRef is an animation handle to a specific vertex in a path.
// Its idx field is populated during emitObjects; capture it with PathRef.VertexAt.
type PathVertexRef struct {
	idx uint64
}

func (v *PathVertexRef) animIdx() uint64            { return v.idx }
func (v *PathVertexRef) animColorIdx() (uint64, bool) { return 0, false }

// internal vertex representations -------------------------------------------

type straightVtx struct {
	x, y   float64
	radius float64
	ref    *PathVertexRef
}

type cubicVtx struct {
	x, y           float64
	inX, inY       float64 // absolute control point
	outX, outY     float64 // absolute control point
	ref            *PathVertexRef
}

type pathVtxEmitter interface {
	emitVertex(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64)
}

// PathRef -----------------------------------------------------------------------

// PathRef is a handle to a custom path shape added to an artboard.
// Vertices are added with LineTo / CubicTo in declaration order.
type PathRef struct {
	name    string
	x, y    float64
	isClosed bool

	vertices   []pathVtxEmitter
	vertexRefs []*PathVertexRef

	opacitySet  bool
	opacity     float64
	rotationSet bool
	rotationDeg float64
	scaleSet    bool
	scaleX, scaleY float64

	fill   *fillConfig
	stroke *strokeConfig

	drawRules []drawRuleConfig

	// populated during emitObjects
	shapeIdx         uint64
	pathIdx          uint64
	solidColorIdx    uint64
	hasSolidColorIdx bool
}

// AnimTarget implementation
func (p *PathRef) animIdx() uint64                { return p.shapeIdx }
func (p *PathRef) animColorIdx() (uint64, bool)   { return p.solidColorIdx, p.hasSolidColorIdx }

// VertexAt returns an animation handle for the i-th vertex (0-indexed, declaration order).
// The handle's index is resolved at emit time; capture it before calling Bytes().
func (p *PathRef) VertexAt(i int) *PathVertexRef {
	if i < len(p.vertexRefs) {
		return p.vertexRefs[i]
	}
	return nil
}

// LineTo adds a straight (corner) vertex at (x, y).
func (p *PathRef) LineTo(x, y float64) *PathRef {
	ref := &PathVertexRef{}
	p.vertices = append(p.vertices, &straightVtx{x: x, y: y, ref: ref})
	p.vertexRefs = append(p.vertexRefs, ref)
	return p
}

// LineToR adds a straight vertex with a corner radius.
func (p *PathRef) LineToR(x, y, radius float64) *PathRef {
	ref := &PathVertexRef{}
	p.vertices = append(p.vertices, &straightVtx{x: x, y: y, radius: radius, ref: ref})
	p.vertexRefs = append(p.vertexRefs, ref)
	return p
}

// CubicTo adds a cubic bezier vertex. x, y is the anchor point; inX, inY is the
// in-handle (absolute artboard coordinates); outX, outY is the out-handle.
// The Cartesian handles are converted to the polar form required by the binary format.
func (p *PathRef) CubicTo(x, y, inX, inY, outX, outY float64) *PathRef {
	ref := &PathVertexRef{}
	p.vertices = append(p.vertices, &cubicVtx{x: x, y: y, inX: inX, inY: inY, outX: outX, outY: outY, ref: ref})
	p.vertexRefs = append(p.vertexRefs, ref)
	return p
}

// Close marks the path as closed (last vertex connects back to first).
func (p *PathRef) Close() *PathRef { p.isClosed = true; return p }

// Fill sets a solid color fill.
func (p *PathRef) Fill(color uint32) *PathRef {
	p.fill = &fillConfig{color: color}
	return p
}

// FillGradient sets a linear gradient fill.
func (p *PathRef) FillGradient(x1, y1, x2, y2 float64, stops ...GradientStop) *PathRef {
	p.fill = &fillConfig{gradient: &gradientConfig{x1: x1, y1: y1, x2: x2, y2: y2, stops: stops}}
	return p
}

// FillRadialGradient sets a radial gradient fill.
func (p *PathRef) FillRadialGradient(cx, cy, ex, ey float64, stops ...GradientStop) *PathRef {
	p.fill = &fillConfig{gradient: &gradientConfig{x1: cx, y1: cy, x2: ex, y2: ey, stops: stops, radial: true}}
	return p
}

// Stroke sets a stroke.
func (p *PathRef) Stroke(width float64, color uint32) *PathRef {
	p.stroke = &strokeConfig{thickness: width, color: color}
	return p
}

// Opacity sets the path shape's opacity (0.0–1.0).
func (p *PathRef) Opacity(v float64) *PathRef { p.opacitySet = true; p.opacity = v; return p }

// Rotation sets the path shape's rotation in degrees (clockwise).
func (p *PathRef) Rotation(degrees float64) *PathRef {
	p.rotationSet = true
	p.rotationDeg = degrees
	return p
}

// Scale sets the path shape's X and Y scale factors.
func (p *PathRef) Scale(sx, sy float64) *PathRef {
	p.scaleSet = true
	p.scaleX = sx
	p.scaleY = sy
	return p
}

// Name sets the path's name (used for targeting and debugging).
func (p *PathRef) Name(n string) *PathRef { p.name = n; return p }

// DrawAbove makes this path render above target in the draw order.
func (p *PathRef) DrawAbove(target *ShapeRef) *PathRef {
	p.drawRules = append(p.drawRules, drawRuleConfig{target: target, placement: PlacementAbove})
	return p
}

// DrawBelow makes this path render below target in the draw order.
func (p *PathRef) DrawBelow(target *ShapeRef) *PathRef {
	p.drawRules = append(p.drawRules, drawRuleConfig{target: target, placement: PlacementBelow})
	return p
}

// emitObjects writes Shape → PointsPath → vertices → paint children into the object list.
func (p *PathRef) emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	// Shape
	p.shapeIdx = uint64(len(*objects)) - artboardOffset
	shape := &rive.Shape{}
	shape.Name = p.name
	shape.ParentId = parentIdx
	shape.X = p.x
	shape.Y = p.y
	shape.Opacity = 1.0
	shape.ScaleX = 1.0
	shape.ScaleY = 1.0
	shape.BlendModeValue = 3
	if p.opacitySet {
		shape.Opacity = p.opacity
	}
	if p.rotationSet {
		shape.Rotation = p.rotationDeg * math.Pi / 180.0
	}
	if p.scaleSet {
		shape.ScaleX = p.scaleX
		shape.ScaleY = p.scaleY
	}
	*objects = append(*objects, shape)

	// PointsPath (child of Shape, at origin relative to shape)
	p.pathIdx = uint64(len(*objects)) - artboardOffset
	pp := &rive.PointsPath{}
	pp.ParentId = p.shapeIdx
	pp.Opacity = 1.0
	pp.ScaleX = 1.0
	pp.ScaleY = 1.0
	pp.IsClosed = p.isClosed
	*objects = append(*objects, pp)

	// Vertices in declaration order
	for _, v := range p.vertices {
		v.emitVertex(objects, p.pathIdx, artboardOffset)
	}

	// Fill paint chain
	if p.fill != nil {
		fillRelIdx := uint64(len(*objects)) - artboardOffset
		fill := &rive.Fill{}
		fill.ParentId = p.shapeIdx
		fill.IsVisible = true
		fill.BlendModeValue = 127
		*objects = append(*objects, fill)

		if p.fill.gradient != nil {
			emitGradient(objects, fillRelIdx, artboardOffset, p.fill.gradient)
		} else {
			p.solidColorIdx = uint64(len(*objects)) - artboardOffset
			p.hasSolidColorIdx = true
			sc := &rive.SolidColor{}
			sc.ColorValue = p.fill.color
			sc.ParentId = fillRelIdx
			*objects = append(*objects, sc)
		}
	}

	// Stroke paint chain
	if p.stroke != nil {
		strokeRelIdx := uint64(len(*objects)) - artboardOffset
		st := &rive.Stroke{}
		st.Thickness = p.stroke.thickness
		st.ParentId = p.shapeIdx
		st.IsVisible = true
		st.BlendModeValue = 127
		*objects = append(*objects, st)

		sc := &rive.SolidColor{}
		sc.ColorValue = p.stroke.color
		sc.ParentId = strokeRelIdx
		*objects = append(*objects, sc)
	}
}

func (p *PathRef) emitDrawRules(objects *[]rive.Object, artboardOffset uint64) {
	for _, rule := range p.drawRules {
		drIdx := uint64(len(*objects)) - artboardOffset
		dtIdx := drIdx + 1

		dr := &rive.DrawRules{}
		dr.ParentId = p.shapeIdx
		dr.DrawTargetId = dtIdx
		*objects = append(*objects, dr)

		dt := &rive.DrawTarget{}
		dt.ParentId = drIdx
		dt.DrawableId = rule.target.shapeIdx
		dt.PlacementValue = rule.placement
		*objects = append(*objects, dt)
	}
}

// vertex emit implementations -----------------------------------------------

func (v *straightVtx) emitVertex(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	v.ref.idx = uint64(len(*objects)) - artboardOffset
	sv := &rive.StraightVertex{}
	sv.ParentId = parentIdx
	sv.X = v.x
	sv.Y = v.y
	if v.radius != 0 {
		sv.Radius = v.radius
	}
	*objects = append(*objects, sv)
}

func (v *cubicVtx) emitVertex(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	v.ref.idx = uint64(len(*objects)) - artboardOffset
	cv := &rive.CubicDetachedVertex{}
	cv.ParentId = parentIdx
	cv.X = v.x
	cv.Y = v.y
	// Convert Cartesian handles to polar (rotation + distance)
	dx, dy := v.inX-v.x, v.inY-v.y
	cv.InRotation = math.Atan2(dy, dx)
	cv.InDistance = math.Sqrt(dx*dx + dy*dy)
	dx, dy = v.outX-v.x, v.outY-v.y
	cv.OutRotation = math.Atan2(dy, dx)
	cv.OutDistance = math.Sqrt(dx*dx + dy*dy)
	*objects = append(*objects, cv)
}

// emitGradient is a package-level helper shared by ShapeRef and PathRef.
func emitGradient(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64, g *gradientConfig) {
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
		lg.Opacity = 1.0
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
