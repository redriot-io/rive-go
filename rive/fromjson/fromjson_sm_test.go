package fromjson_test

// fromjson_sm_test.go — unit tests for the FromJSON state machine features:
//   (1) bool/number/trigger inputs + number defaults
//   (2) BlendState1D states
//   (3) transition duration_ms and exit_time
//   (4) ExitState as transition target
//   (5) set_number listener action
//   (6) round-trip interactive button via JSON

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// minimalSMScene wraps a state machine JSON fragment into a full scene.
// The artboard has one rectangle and two hold animations so states can
// reference them, plus whichever state_machines block is passed in.
func minimalSMScene(smBlock string) string {
	return `{
		"version": 1,
		"artboard": {
			"name": "Test", "width": 400, "height": 200,
			"children": [{"type":"rectangle","name":"btn","x":200,"y":100,"width":100,"height":60,"fill":"#1565C0"}],
			"animations": [
				{"name":"idle","duration":0.033,"fps":60,"tracks":[]},
				{"name":"active","duration":0.033,"fps":60,"tracks":[]}
			],
			"state_machines": [` + smBlock + `]
		}
	}`
}

// ── 1. Inputs ─────────────────────────────────────────────────────────────────

func TestSM_BoolInput(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"toggle","type":"bool"}],
		"layers": [{"name":"L","states":[{"name":"A"}],"transitions":[]}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// StateMachineBool typeKey=59
	if n := countType(f.Objects, 59); n != 1 {
		t.Errorf("want 1 StateMachineBool, got %d", n)
	}
}

func TestSM_TriggerInput(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"fire","type":"trigger"}],
		"layers": [{"name":"L","states":[{"name":"A"}]}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// StateMachineTrigger typeKey=58 (check registry)
	if n := countType(f.Objects, 58); n != 1 {
		t.Errorf("want 1 StateMachineTrigger (typeKey 58), got %d", n)
	}
}

func TestSM_NumberInputDefault(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"speed","type":"number","default":0.75}],
		"layers": [{"name":"L","states":[{"name":"A"}]}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// StateMachineNumber typeKey=60
	nums := collectType(f.Objects, 56)
	if len(nums) != 1 {
		t.Fatalf("want 1 StateMachineNumber, got %d", len(nums))
	}
	// Key 152 is the value property for StateMachineNumber
	props := propsByKey(nums[0].Properties())
	v, ok := props[140]
	if !ok {
		t.Fatal("StateMachineNumber value (key 152) not emitted — default may not be applied")
	}
	if got := v.Value.(float64); got != 0.75 {
		t.Errorf("number input default: got %v, want 0.75", got)
	}
}

func TestSM_NumberInputZeroDefault(t *testing.T) {
	// Default 0 is the runtime default, so it should be suppressed (not emitted).
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"speed","type":"number"}],
		"layers": [{"name":"L","states":[{"name":"A"}]}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)
	nums := collectType(f.Objects, 56)
	if len(nums) != 1 {
		t.Fatalf("want 1 StateMachineNumber, got %d", len(nums))
	}
	props := propsByKey(nums[0].Properties())
	if _, ok := props[140]; ok {
		t.Error("StateMachineNumber value (key 152) should be suppressed when default is 0")
	}
}

// ── 2. BlendState1D ───────────────────────────────────────────────────────────

func TestSM_BlendState1D_Emitted(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Test", "width": 420, "height": 160,
			"children": [{"type":"rectangle","name":"r","x":60,"y":80,"width":60,"height":60,"fill":"#1E88E5"}],
			"animations": [
				{"name":"walk","duration":1.0,"fps":60,"loop":"pingpong","tracks":[
					{"target":"r.x","keyframes":[{"time":0,"value":60},{"time":1,"value":100}]}
				]},
				{"name":"run","duration":1.0,"fps":60,"loop":"pingpong","tracks":[
					{"target":"r.x","keyframes":[{"time":0,"value":60},{"time":1,"value":360}]}
				]}
			],
			"state_machines": [{
				"name": "SpeedSM",
				"inputs": [{"name":"speed","type":"number","default":0.5}],
				"layers": [{
					"name": "Main",
					"states": [{
						"name": "SpeedBlend",
						"type": "blend_1d",
						"input": "speed",
						"blends": [
							{"animation":"walk","threshold":0.0},
							{"animation":"run","threshold":1.0}
						]
					}]
				}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// BlendState1DInput typeKey=76
	if n := countType(f.Objects, 76); n != 1 {
		t.Errorf("want 1 BlendState1DInput (typeKey 76), got %d", n)
	}
	// BlendAnimation1D typeKey=75: one per threshold entry
	if n := countType(f.Objects, 75); n != 2 {
		t.Errorf("want 2 BlendAnimation1D (typeKey 75), got %d", n)
	}
}

func TestSM_BlendState1D_InputId(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Test", "width": 420, "height": 160,
			"children": [{"type":"rectangle","name":"r","x":60,"y":80,"width":60,"height":60,"fill":"#1E88E5"}],
			"animations": [
				{"name":"slow","duration":1.0,"fps":60,"tracks":[]},
				{"name":"fast","duration":1.0,"fps":60,"tracks":[]}
			],
			"state_machines": [{
				"name": "SpeedSM",
				"inputs": [
					{"name":"unused","type":"bool"},
					{"name":"speed","type":"number"}
				],
				"layers": [{
					"name": "Main",
					"states": [{
						"name": "B",
						"type": "blend_1d",
						"input": "speed",
						"blends": [
							{"animation":"slow","threshold":0.0},
							{"animation":"fast","threshold":1.0}
						]
					}]
				}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	bs := collectType(f.Objects, 76)
	if len(bs) != 1 {
		t.Fatalf("want 1 BlendState1DInput, got %d", len(bs))
	}
	// speed is input index 1 (after unused bool)
	props := propsByKey(bs[0].Properties())
	v, ok := props[167] // inputId key
	if !ok {
		t.Fatal("BlendState1DInput.inputId (key 167) not emitted")
	}
	if got := v.Value.(uint64); got != 1 {
		t.Errorf("BlendState1DInput.inputId = %d, want 1 (speed is 2nd input)", got)
	}
}

func TestSM_BlendState1D_UnknownInput_Error(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Test", "width": 400, "height": 200,
			"children": [{"type":"rectangle","name":"r","x":200,"y":100,"width":100,"height":60,"fill":"#F00"}],
			"state_machines": [{
				"name": "SM",
				"inputs": [{"name":"speed","type":"number"}],
				"layers": [{"name":"L","states":[{
					"name":"B","type":"blend_1d","input":"nonexistent","blends":[]
				}]}]
			}]
		}
	}`
	_, err := fromjson.FromJSON([]byte(scene))
	if err == nil {
		t.Fatal("expected error for unknown blend_1d input")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("error should name the unknown input, got: %v", err)
	}
}

// ── 3. Transition timing ──────────────────────────────────────────────────────

func TestSM_TransitionDuration(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"go","type":"bool"}],
		"layers": [{
			"name": "L",
			"states": [
				{"name":"Idle","animation":"idle"},
				{"name":"Active","animation":"active"}
			],
			"transitions": [{
				"from":"Idle","to":"Active",
				"duration_ms": 200,
				"conditions":[{"input":"go","value":true}]
			}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	transitions := collectType(f.Objects, 65) // StateTransition typeKey=64
	if len(transitions) == 0 {
		t.Fatal("no StateTransition objects found")
	}
	// Find the user-defined transition (entry transition has stateToId=3, user trans has conditions)
	// Duration key=65 on StateTransition; 200ms
	found := false
	for _, tr := range transitions {
		props := propsByKey(tr.Properties())
		if v, ok := props[158]; ok {
			if v.Value.(uint64) == 200 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a StateTransition with duration=200ms (key 65)")
	}
}

func TestSM_TransitionExitTime(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"go","type":"bool"}],
		"layers": [{
			"name": "L",
			"states": [
				{"name":"Idle","animation":"idle"},
				{"name":"Active","animation":"active"}
			],
			"transitions": [{
				"from":"Idle","to":"Active",
				"exit_time": 30,
				"conditions":[{"input":"go","value":true}]
			}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	transitions := collectType(f.Objects, 65)
	found := false
	for _, tr := range transitions {
		props := propsByKey(tr.Properties())
		if v, ok := props[160]; ok { // exitTime key=160
			if v.Value.(uint64) == 30 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a StateTransition with exitTime=30 frames (key 66)")
	}
}

// ── 4. ExitState ──────────────────────────────────────────────────────────────

func TestSM_ExitState_AsTransitionTarget(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"done","type":"bool"}],
		"layers": [{
			"name": "L",
			"states": [{"name":"Play","animation":"idle"}],
			"transitions": [{
				"from":"Play","to":"ExitState",
				"conditions":[{"input":"done","value":true}]
			}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// Transition should exist pointing to state index 2 (ExitState sentinel)
	transitions := collectType(f.Objects, 65)
	found := false
	for _, tr := range transitions {
		props := propsByKey(tr.Properties())
		if v, ok := props[151]; ok { // stateToId key=151
			if v.Value.(uint64) == 2 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a StateTransition with stateToId=2 (ExitState sentinel)")
	}
}

func TestSM_ExitState_CaseInsensitive(t *testing.T) {
	// "exitstate" (lowercase) should also work
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"done","type":"trigger"}],
		"layers": [{
			"name": "L",
			"states": [{"name":"Play","animation":"idle"}],
			"transitions": [{"from":"Play","to":"exitstate","conditions":[{"input":"done"}]}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	if _, err := rive.ReadBytes(data); err != nil {
		t.Fatalf("ReadBytes with lowercase exitstate: %v", err)
	}
}

// ── 5. set_number listener action ─────────────────────────────────────────────

func TestSM_Listener_SetNumber(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"speed","type":"number"}],
		"layers": [{"name":"L","states":[{"name":"A"}]}],
		"listeners": [{
			"target": "btn",
			"event": "pointer_down",
			"actions": [{"type":"set_number","input":"speed","value":1.0}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// ListenerNumberChange typeKey=118
	numChanges := collectType(f.Objects, 118)
	if len(numChanges) != 1 {
		t.Fatalf("want 1 ListenerNumberChange (typeKey 118), got %d", len(numChanges))
	}
	props := propsByKey(numChanges[0].Properties())
	// value key=229
	if v, ok := props[229]; !ok || v.Value.(float64) != 1.0 {
		t.Errorf("ListenerNumberChange.value = %v, want 1.0", props[229].Value)
	}
}

func TestSM_Listener_SetNumber_Zero(t *testing.T) {
	// set_number with value=0 should emit a ListenerNumberChange with value suppressed
	// (key 229 only emitted when != 0 per gen_animation.go Properties())
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"speed","type":"number"}],
		"layers": [{"name":"L","states":[{"name":"A"}]}],
		"listeners": [{
			"target": "btn",
			"event": "pointer_up",
			"actions": [{"type":"set_number","input":"speed","value":0}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// ListenerNumberChange should still be emitted (the object exists, value prop suppressed)
	numChanges := collectType(f.Objects, 118)
	if len(numChanges) != 1 {
		t.Fatalf("want 1 ListenerNumberChange, got %d", len(numChanges))
	}
}

// ── 6. Round-trip: full interactive button from JSON ──────────────────────────

func TestSM_RoundTrip_InteractiveButton(t *testing.T) {
	// Full interactive button matching the builder-generated one:
	// hover + pressed booleans, pointer enter/exit/down/up listeners.
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Interactive Button", "width": 300, "height": 120,
			"children": [{"type":"rectangle","name":"button","x":150,"y":60,"width":260,"height":80,
				"fill":"#1565C0","corner_radius":12}],
			"animations": [
				{"name":"idle_anim",   "duration":0.033,"fps":60,"tracks":[]},
				{"name":"hover_anim",  "duration":0.033,"fps":60,"tracks":[]},
				{"name":"pressed_anim","duration":0.033,"fps":60,"tracks":[]}
			],
			"state_machines": [{
				"name": "ButtonSM",
				"inputs": [
					{"name":"hovered","type":"bool"},
					{"name":"pressed","type":"bool"}
				],
				"layers": [{
					"name": "Main",
					"states": [
						{"name":"Idle",    "animation":"idle_anim"},
						{"name":"Hover",   "animation":"hover_anim"},
						{"name":"Pressed", "animation":"pressed_anim"}
					],
					"transitions": [
						{"from":"Idle",    "to":"Hover",    "conditions":[{"input":"hovered","value":true}]},
						{"from":"Hover",   "to":"Idle",     "conditions":[{"input":"hovered","value":false}]},
						{"from":"Idle",    "to":"Pressed",  "conditions":[{"input":"pressed","value":true}]},
						{"from":"Hover",   "to":"Pressed",  "conditions":[{"input":"pressed","value":true}]},
						{"from":"Pressed", "to":"Hover",    "conditions":[{"input":"pressed","value":false},{"input":"hovered","value":true}]},
						{"from":"Pressed", "to":"Idle",     "conditions":[{"input":"pressed","value":false}]}
					]
				}],
				"listeners": [
					{"target":"button","event":"pointer_enter","actions":[{"type":"set_bool","input":"hovered","value":true}]},
					{"target":"button","event":"pointer_exit", "actions":[{"type":"set_bool","input":"hovered","value":false},{"type":"set_bool","input":"pressed","value":false}]},
					{"target":"button","event":"pointer_down", "actions":[{"type":"set_bool","input":"pressed","value":true}]},
					{"target":"button","event":"pointer_up",   "actions":[{"type":"set_bool","input":"pressed","value":false}]}
				]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// 1 StateMachine, 2 bool inputs, 6 transitions, 4 listeners, 5 bool changes
	if n := countType(f.Objects, 53); n != 1 {
		t.Errorf("StateMachine count: got %d, want 1", n)
	}
	if n := countType(f.Objects, 59); n != 2 {
		t.Errorf("StateMachineBool count: got %d, want 2", n)
	}
	// 4 StateMachineListenerSingle typeKey=654 → check listeners present
	if n := countType(f.Objects, 114); n != 4 {
		t.Errorf("StateMachineListenerSingle count: got %d, want 4", n)
	}
	// ListenerBoolChange typeKey=117: 5 total (enter:1, exit:2, down:1, up:1)
	if n := countType(f.Objects, 117); n != 5 {
		t.Errorf("ListenerBoolChange count: got %d, want 5", n)
	}
}

// ── 7. Number condition op ─────────────────────────────────────────────────────

func TestSM_NumberCondition_GreaterThan(t *testing.T) {
	scene := minimalSMScene(`{
		"name": "SM",
		"inputs": [{"name":"speed","type":"number"}],
		"layers": [{
			"name": "L",
			"states": [
				{"name":"Walk","animation":"idle"},
				{"name":"Run","animation":"active"}
			],
			"transitions": [{
				"from":"Walk","to":"Run",
				"conditions":[{"input":"speed","value":0.5,"op":">"}]
			}]
		}]
	}`)
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// TransitionNumberCondition typeKey=68
	numConds := collectType(f.Objects, 70)
	if len(numConds) == 0 {
		t.Fatal("no TransitionNumberCondition found")
	}
	props := propsByKey(numConds[0].Properties())
	// op key=182: GreaterThan=3
	if v, ok := props[156]; !ok || v.Value.(uint64) != 3 {
		t.Errorf("condition op = %v, want 3 (GreaterThan)", props[156].Value)
	}
	// value key=183: 0.5
	if v, ok := props[157]; !ok || v.Value.(float64) != 0.5 {
		t.Errorf("condition value = %v, want 0.5", props[157].Value)
	}
}
