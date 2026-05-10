// Code generated from internal/schema/defs — verified against JSON key definitions.
package builder

// Common property keys for animation targeting.
// Source: internal/schema/defs/node.json, transform_component.json,
// world_transform_component.json, shapes/solid_color.json
const (
	// Node / TransformComponent properties (shape-level, animate the Shape object)
	PropX        uint32 = 13 // Node.x (double)
	PropY        uint32 = 14 // Node.y (double)
	PropRotation uint32 = 15 // TransformComponent.rotation (double, radians)
	PropScaleX   uint32 = 16 // TransformComponent.scaleX (double, default 1.0)
	PropScaleY   uint32 = 17 // TransformComponent.scaleY (double, default 1.0)
	PropOpacity  uint32 = 18 // WorldTransformComponent.opacity (double, 0.0–1.0)

	// ParametricPath properties (animate the Rectangle/Ellipse path child)
	PropWidth  uint32 = 20 // ParametricPath.width
	PropHeight uint32 = 21 // ParametricPath.height

	// SolidColor property (animate the SolidColor child of a Fill)
	PropColorValue uint32 = 37 // SolidColor.colorValue (color)

	// DrawTarget / DrawRules properties (draw order control)
	PropDrawableId    uint32 = 119 // DrawTarget.drawableId — artboard-relative index of target shape
	PropPlacementValue uint32 = 120 // DrawTarget.placementValue — 0=above, 1=below
	PropDrawTargetId  uint32 = 121 // DrawRules.drawTargetId — artboard-relative index of DrawTarget
)
