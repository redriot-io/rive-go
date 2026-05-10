package builder

// Placement constants for DrawTarget.PlacementValue.
const (
	PlacementAbove uint64 = 0 // render source shape in front of target
	PlacementBelow uint64 = 1 // render source shape behind target
)

// drawRuleConfig is one draw-order constraint attached to a ShapeRef.
// source is the shape that owns this rule; target is the reference shape.
type drawRuleConfig struct {
	target    *ShapeRef
	placement uint64
}
