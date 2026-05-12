package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/redriot-io/rive-go/rive"
)

// cmdVerify runs structural checks on a .riv file and exits 0 iff all pass.
// When deep is true, also parses embedded font cmap tables and verifies that
// every TextValueRun's text is covered by its assigned font's glyphs.
func cmdVerify(path string, deep bool) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("✗ Cannot read file: %v\n", err)
		fmt.Println("Result: FAIL (1 error)")
		return false
	}

	f, err := rive.ReadBytes(raw)
	if err != nil {
		fmt.Printf("✗ Parse error: %v\n", err)
		fmt.Println("Result: FAIL (1 error)")
		return false
	}

	passes, errs := verifyRiv(f)

	if deep {
		fp, fe := verifyFonts(f)
		passes = append(passes, fp...)
		for _, e := range fe {
			if strings.HasPrefix(e, "⚠") {
				// partial coverage — print as warning, not a hard failure
				fmt.Printf("%s\n", e)
			} else {
				errs = append(errs, e)
			}
		}

		ip, ie := verifyImages(f)
		passes = append(passes, ip...)
		for _, e := range ie {
			if strings.HasPrefix(e, "\u26a0") {
				fmt.Printf("%s\n", e)
			} else {
				errs = append(errs, e)
			}
		}
	}

	for _, p := range passes {
		fmt.Printf("✓ %s\n", p)
	}
	for _, e := range errs {
		fmt.Printf("✗ %s\n", e)
	}

	if len(errs) > 0 {
		fmt.Printf("Result: FAIL (%d error(s))\n", len(errs))
		return false
	}
	fmt.Printf("Result: PASS (%d checks)\n", len(passes))
	return true
}

// verifyRiv performs structural checks: parentId refs, KeyedObject refs,
// DrawRules chains, and SM wiring (anim refs, input refs, listener wiring).
func verifyRiv(f *rive.File) (passes, errs []string) {
	pass := func(msg string) { passes = append(passes, msg) }
	fail := func(msg string) { errs = append(errs, msg) }

	objs := f.Objects

	// Locate Artboard
	artboardIdx := -1
	for i, o := range objs {
		if o.TypeKey() == 1 {
			artboardIdx = i
			break
		}
	}
	if artboardIdx < 0 {
		fail("no Artboard object found")
		return
	}
	pass(fmt.Sprintf("Artboard at index %d", artboardIdx))

	// Count LinearAnimations (typeKey=31) for animationId bounds checks
	animCount := 0
	for _, o := range objs {
		if o.TypeKey() == 31 {
			animCount++
		}
	}
	if animCount > 0 {
		pass(fmt.Sprintf("%d animation(s)", animCount))
	}

	// 1. parentId (key=5) resolution — artboard-relative
	{
		n, bad := 0, 0
		for i, o := range objs {
			if v, ok := verifyGetUint(o, 5); ok {
				n++
				rel := int(v)
				global := artboardIdx + rel
				if global < 0 || global >= len(objs) || global == i {
					fail(fmt.Sprintf("Object[%d] (typeKey=%d) parentId=%d → global=%d invalid", i, o.TypeKey(), rel, global))
					bad++
				}
			}
		}
		if bad == 0 {
			pass(fmt.Sprintf("parentId references valid (%d)", n))
		}
	}

	// 2. KeyedObject (typeKey=25) .objectId (key=51) bounds
	{
		n, bad := 0, 0
		for i, o := range objs {
			if o.TypeKey() != 25 {
				continue
			}
			if v, ok := verifyGetUint(o, 51); ok {
				n++
				if int(v) >= len(objs) {
					fail(fmt.Sprintf("Object[%d] KeyedObject.objectId=%d out of range [0,%d)", i, v, len(objs)))
					bad++
				}
			}
		}
		if bad == 0 && n > 0 {
			pass(fmt.Sprintf("KeyedObject.objectId references valid (%d)", n))
		}
	}

	// 3. DrawRules (typeKey=49) .drawTargetId (key=121) → DrawTarget (typeKey=48)
	{
		n, bad := 0, 0
		for i, o := range objs {
			if o.TypeKey() != 49 {
				continue
			}
			if v, ok := verifyGetUint(o, 121); ok {
				n++
				global := artboardIdx + int(v)
				if global < 0 || global >= len(objs) || objs[global].TypeKey() != 48 {
					var got uint32
					if global >= 0 && global < len(objs) {
						got = objs[global].TypeKey()
					}
					fail(fmt.Sprintf("Object[%d] DrawRules.drawTargetId=%d → typeKey=%d (expected DrawTarget=48)", i, v, got))
					bad++
				}
			}
		}
		if bad == 0 && n > 0 {
			pass(fmt.Sprintf("DrawRules chain valid (%d)", n))
		}
	}

	// 4. DrawTarget (typeKey=48) .drawableId (key=119) bounds
	{
		n, bad := 0, 0
		for i, o := range objs {
			if o.TypeKey() != 48 {
				continue
			}
			if v, ok := verifyGetUint(o, 119); ok {
				n++
				global := artboardIdx + int(v)
				if global < 0 || global >= len(objs) {
					fail(fmt.Sprintf("Object[%d] DrawTarget.drawableId=%d out of range", i, v))
					bad++
				}
			}
		}
		if bad == 0 && n > 0 {
			pass(fmt.Sprintf("DrawTarget.drawableId references valid (%d)", n))
		}
	}

	// 5. State machine wiring
	smErrs, smPasses := verifySMs(objs, artboardIdx, animCount)
	passes = append(passes, smPasses...)
	errs = append(errs, smErrs...)

	return
}

// verifySMs scans the object stream sequentially, maintaining per-SM context to
// validate animationId refs, transition condition inputIds, listener targetIds,
// the T-423 eventId bug, and listener action inputIds.
func verifySMs(objs []rive.Object, artboardIdx, animCount int) (errs, passes []string) {
	pass := func(msg string) { passes = append(passes, msg) }
	fail := func(msg string) { errs = append(errs, msg) }

	smCount := 0
	totalListeners := 0
	totalConditions := 0

	smInputCount := 0
	smName := ""
	inSM := false

	for i, o := range objs {
		if i <= artboardIdx {
			continue
		}

		tk := o.TypeKey()

		if tk == 53 { // StateMachine — reset per-SM context
			smCount++
			inSM = true
			smInputCount = 0
			smName = fmt.Sprintf("SM[%d]", i)
			for _, p := range o.Properties() {
				if p.Key == 4 {
					if s, ok := p.Value.(string); ok {
						smName = fmt.Sprintf("SM[%d] %q", i, s)
					}
				}
			}
			continue
		}

		if !inSM {
			continue
		}

		switch tk {
		case 58, 59, 56: // StateMachineTrigger, StateMachineBool, StateMachineNumber
			smInputCount++

		case 61: // AnimationState — check animationId (key=149) < animCount
			if v, ok := verifyGetUint(o, 149); ok {
				if int(v) >= animCount {
					fail(fmt.Sprintf("%s AnimationState[%d] animationId=%d out of range (have %d animation(s))", smName, i, v, animCount))
				}
			}

		case 71, 67, 68, 70: // TransitionBool/Input/Trigger/NumberCondition
			// check inputId (key=155) < smInputCount
			totalConditions++
			if v, ok := verifyGetUint(o, 155); ok {
				if int(v) >= smInputCount {
					fail(fmt.Sprintf("%s TransitionCondition[%d] inputId=%d >= smInputCount=%d", smName, i, v, smInputCount))
				}
			}

		case 114: // StateMachineListenerSingle
			totalListeners++
			// eventId=0 (key=399) is the T-423 bug: Go zero-value gets emitted,
			// Rive treats the listener as a Rive Event listener so pointer events
			// never fire. Nonzero eventId is an intentional Rive Event listener.
			if ev, ok := verifyGetUint(o, 399); ok && ev == 0 {
				fail(fmt.Sprintf("%s Listener[%d] eventId=0 emitted — pointer listener will not fire; set EventId=^uint64(0) to suppress (T-423 bug)", smName, i))
			}
			// targetId (key=224) must be in-bounds.
			// For pointer listeners (no nonzero eventId), target must be a Shape (typeKey=3).
			isEventListener := func() bool {
				ev, ok := verifyGetUint(o, 399)
				return ok && ev != 0
			}()
			if v, ok := verifyGetUint(o, 224); ok {
				global := artboardIdx + int(v)
				if global < 0 || global >= len(objs) {
					fail(fmt.Sprintf("%s Listener[%d] targetId=%d out of range", smName, i, v))
				} else if !isEventListener && objs[global].TypeKey() != 3 {
					fail(fmt.Sprintf("%s Listener[%d] targetId=%d → typeKey=%d (expected Shape=3 for pointer listener)", smName, i, v, objs[global].TypeKey()))
				}
			}

		case 115, 117: // ListenerTriggerChange, ListenerBoolChange
			// inputId (key=227) must be < smInputCount
			if v, ok := verifyGetUint(o, 227); ok {
				if int(v) >= smInputCount {
					fail(fmt.Sprintf("%s ListenerAction[%d] inputId=%d >= smInputCount=%d", smName, i, v, smInputCount))
				}
			}
		}
	}

	if smCount == 0 {
		pass("No state machines")
	} else {
		pass(fmt.Sprintf("%d SM(s): %d listener(s), %d transition condition(s) verified", smCount, totalListeners, totalConditions))
	}

	return
}

// verifyGetUint returns the uint64 value for the first matching property key.
func verifyGetUint(o rive.Object, key uint32) (uint64, bool) {
	for _, p := range o.Properties() {
		if p.Key == key {
			if v, ok := p.Value.(uint64); ok {
				return v, true
			}
		}
	}
	return 0, false
}

// verifyHasProp reports whether a property key appears in the object's stream.
func verifyHasProp(o rive.Object, key uint32) bool {
	for _, p := range o.Properties() {
		if p.Key == key {
			return true
		}
	}
	return false
}
