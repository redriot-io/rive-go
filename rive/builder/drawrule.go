package builder

// PlacementAbove and PlacementBelow are values for DrawTarget.PlacementValue (Rive key 120).
// They mirror the PlacementValue enum in the Rive runtime's draw_target.cpp.
//
//   - PlacementAbove (0): the source shape renders in front of (on top of) the target drawable.
//   - PlacementBelow (1): the source shape renders behind the target drawable.
//
// Use with DrawAbove / DrawBelow (static) or KeyframeDrawOrder (animated).
const (
	PlacementAbove uint64 = 0 // source renders in front of target (Rive: "above")
	PlacementBelow uint64 = 1 // source renders behind target (Rive: "below")
)

// drawRuleConfig is one draw-order constraint attached to a ShapeRef.
// source is the shape that owns this rule; target is the reference shape.
type drawRuleConfig struct {
	target    *ShapeRef
	placement uint64
}
