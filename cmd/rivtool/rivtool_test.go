package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execDir is a writable+executable directory for test binaries.
// /tmp is mounted noexec in this environment.
const execDir = "/app/workspace/tmp"

// buildRivtool compiles rivtool into a temp binary and returns its path.
func buildRivtool(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(execDir, "rivtool-test-*")
	if err != nil {
		t.Fatalf("mktemp in %s: %v", execDir, err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	bin := filepath.Join(dir, "rivtool")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GOTMPDIR="+execDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build rivtool: %v\n%s", err, out)
	}
	return bin
}

func run(t *testing.T, bin string, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

func runWithStdin(t *testing.T, bin string, input []byte, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

// ── create --from <file> ──────────────────────────────────────────────────────

func TestCreate_Simple_Stdout(t *testing.T) {
	bin := buildRivtool(t)
	stdout, stderr, code := run(t, bin, "create", "--from", "testdata/fromjson/simple.json")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	// Output should start with RIVE fingerprint
	if len(stdout) < 4 || string(stdout[:4]) != "RIVE" {
		t.Fatalf("stdout does not start with RIVE fingerprint, got %q (len=%d)", stdout[:min(16, len(stdout))], len(stdout))
	}
}

func TestCreate_Animated_Stdout(t *testing.T) {
	bin := buildRivtool(t)
	stdout, stderr, code := run(t, bin, "create", "--from", "testdata/fromjson/animated.json")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if string(stdout[:4]) != "RIVE" {
		t.Fatalf("stdout not a .riv file")
	}
}

func TestCreate_StateMachine_Stdout(t *testing.T) {
	bin := buildRivtool(t)
	stdout, stderr, code := run(t, bin, "create", "--from", "testdata/fromjson/state_machine.json")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if string(stdout[:4]) != "RIVE" {
		t.Fatalf("stdout not a .riv file")
	}
}

func TestCreate_OutputFile(t *testing.T) {
	bin := buildRivtool(t)
	dir, err := os.MkdirTemp(execDir, "rivtool-out-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	out := filepath.Join(dir, "out.riv")
	_, stderr, code := run(t, bin, "create", "--from", "testdata/fromjson/simple.json", "--output", out)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data[:4]) != "RIVE" {
		t.Fatalf("output file does not start with RIVE")
	}
	// stderr should mention the output path
	if !strings.Contains(string(stderr), "out.riv") {
		t.Errorf("stderr should mention output path, got: %s", stderr)
	}
}

// ── create --from - (stdin) ───────────────────────────────────────────────────

func TestCreate_Stdin(t *testing.T) {
	bin := buildRivtool(t)
	scene := `{"version":1,"artboard":{"name":"A","width":200,"height":200,
		"children":[{"type":"rectangle","name":"r","x":100,"y":100,"width":50,"height":50,"fill":"#FF0000"}]}}`
	stdout, stderr, code := runWithStdin(t, bin, []byte(scene), "create", "--from", "-")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if string(stdout[:4]) != "RIVE" {
		t.Fatalf("stdin output not a .riv file")
	}
}

// ── create error cases ────────────────────────────────────────────────────────

func TestCreate_InvalidJSON_ExitsOne(t *testing.T) {
	bin := buildRivtool(t)
	_, stderr, code := run(t, bin, "create", "--from", "testdata/fromjson/invalid.json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if len(stderr) == 0 {
		t.Error("expected error message on stderr")
	}
}

func TestCreate_MissingFile_ExitsOne(t *testing.T) {
	bin := buildRivtool(t)
	_, stderr, code := run(t, bin, "create", "--from", "nonexistent.json")
	if code != 1 {
		t.Fatalf("expected exit 1 for missing file, got %d", code)
	}
	if len(stderr) == 0 {
		t.Error("expected error on stderr")
	}
}

func TestCreate_MissingFromFlag_ExitsOne(t *testing.T) {
	bin := buildRivtool(t)
	_, _, code := run(t, bin, "create")
	if code != 1 {
		t.Fatalf("expected exit 1 when --from is missing, got %d", code)
	}
}

func TestCreate_BadEasingSyntax_ExitsOne(t *testing.T) {
	bin := buildRivtool(t)
	scene := `{"version":1,"artboard":{"name":"A","width":200,"height":200,
		"children":[{"type":"rectangle","name":"r","x":100,"y":100,"width":50,"height":50,"fill":"#FF0000"}],
		"animations":[{"name":"a","duration":1,"fps":60,"tracks":[
			{"target":"r.x","keyframes":[{"time":0,"value":0,"easing":"magic-easing"},{"time":1,"value":100}]}
		]}]}}`
	stdout, stderr, code := runWithStdin(t, bin, []byte(scene), "create", "--from", "-")
	_ = stdout
	if code != 1 {
		t.Fatalf("expected exit 1 for bad easing, got %d (stderr: %s)", code, stderr)
	}
}

// ── validate --schema ─────────────────────────────────────────────────────────

func TestValidateSchema_Valid(t *testing.T) {
	bin := buildRivtool(t)
	stdout, stderr, code := run(t, bin, "validate", "--schema", "testdata/fromjson/simple.json")
	_ = stderr
	if code != 0 {
		t.Fatalf("exit %d for valid JSON: stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(string(stdout), "VALID") {
		t.Errorf("expected VALID in output, got: %s", stdout)
	}
}

func TestValidateSchema_Invalid(t *testing.T) {
	bin := buildRivtool(t)
	stdout, _, code := run(t, bin, "validate", "--schema", "testdata/fromjson/invalid.json")
	if code != 1 {
		t.Fatalf("expected exit 1 for invalid JSON schema, got %d", code)
	}
	if !strings.Contains(string(stdout), "INVALID") {
		t.Errorf("expected INVALID in output, got: %s", stdout)
	}
}

// ── validate (existing .riv) still works ────────────────────────────────────

func TestValidate_RivFile_StillWorks(t *testing.T) {
	// Build a .riv from the simple JSON, then validate it
	bin := buildRivtool(t)
	out := filepath.Join(t.TempDir(), "simple.riv")
	_, stderr, code := run(t, bin, "create", "--from", "testdata/fromjson/simple.json", "--output", out)
	if code != 0 {
		t.Fatalf("create: exit %d, stderr: %s", code, stderr)
	}

	stdout, _, code2 := run(t, bin, "validate", out)
	if code2 != 0 {
		t.Fatalf("validate: exit %d, stdout: %s", code2, stdout)
	}
	if !strings.Contains(string(stdout), "VALID") {
		t.Errorf("expected VALID in validate output, got: %s", stdout)
	}
}

// ── helper ────────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
