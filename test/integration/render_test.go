//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"

	"github.com/redriot-io/rive-go/rive/builder"
)

// RiveResult mirrors the JSON written to #result by harness.html.
type RiveResult struct {
	Loaded      bool   `json:"loaded"`
	Error       string `json:"error"`
	Frames      int    `json:"frames"`
	CanvasBlank bool   `json:"canvasBlank"`
}

// testCase pairs a display name, a URL path on the local server, and whether
// we expect the file to load successfully.
type testCase struct {
	name       string
	urlPath    string // absolute path served by the test HTTP server
	wantLoaded bool
}

func TestRiveIntegration(t *testing.T) {
	repoRoot := findRepoRoot(t)

	// 1. Generate fresh .riv files from the builder into a temp dir
	tmpDir := t.TempDir()
	generatedNames := generateTestFiles(t, tmpDir)

	// 2. HTTP server serving:
	//    /          → repo root  (harness.html, fixtures, docs/preview/examples/)
	//    /generated/ → temp dir   (freshly generated files)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(repoRoot)))
	mux.Handle("/generated/", http.StripPrefix("/generated/", http.FileServer(http.Dir(tmpDir))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 3. Launch headless Chromium
	browserURL := launchBrowser(t)
	browser := rod.New().ControlURL(browserURL).MustConnect()
	defer browser.MustClose()

	// 4. Build test cases
	cases := []testCase{
		// Reference: official Rive runtime test file
		{"reference/ball_test.riv (official)", "/test/fixtures/ball_test.riv", true},
		{"reference/reference.riv (docs copy)", "/docs/preview/examples/reference.riv", true},
	}
	// Builder-generated files (freshly built during this test run)
	for _, name := range generatedNames {
		cases = append(cases, testCase{
			name:       "builder/" + name,
			urlPath:    "/generated/" + name,
			wantLoaded: true,
		})
	}
	// Checked-in generated files (from last commit)
	for _, name := range []string{
		"fade_rect.riv",
		"bounce_ball.riv",
		"color_cycle.riv",
		"toggle_button.riv",
		"gradient_ellipse.riv",
		"multi_shape.riv",
	} {
		cases = append(cases, testCase{
			name:       "committed/" + name,
			urlPath:    "/docs/preview/examples/" + name,
			wantLoaded: true,
		})
	}

	// FromJSON-generated files (fromjson pipeline validation)
	for _, name := range []string{
		"fromjson_spin.riv",
		"fromjson_dots.riv",
		"fromjson_gradient.riv",
		"fromjson_color.riv",
		"fromjson_easing.riv",
		"fromjson_autumn.riv",
	} {
		cases = append(cases, testCase{
			name:       "fromjson/" + name,
			urlPath:    "/docs/preview/" + name,
			wantLoaded: true,
		})
	}

	const animDurationMs = 2500

	// 5. Run each test case
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			harnessURL := fmt.Sprintf(
				"%s/test/integration/harness.html?file=%s&duration=%d",
				srv.URL, tc.urlPath, animDurationMs,
			)

			page := browser.MustPage(harnessURL)
			defer page.MustClose()

			// Wait until harness.html sets #result content (JS condition poll)
			waitDur := time.Duration(animDurationMs)*time.Millisecond + 12*time.Second
			err := rod.Try(func() {
				page.Timeout(waitDur).MustWait(`
					() => {
						const el = document.getElementById('result');
						return el != null && el.textContent.trim() !== '';
					}
				`)
			})
			if err != nil {
				status := "(unknown)"
				if el, e := page.Element("#status"); e == nil {
					status, _ = el.Text()
				}
				t.Errorf("TIMEOUT waiting for result (status=%q): %v", status, err)
				return
			}

			resultText := ""
			if el, e := page.Element("#result"); e == nil {
				resultText, _ = el.Text()
			}

			var result RiveResult
			if jsonErr := json.Unmarshal([]byte(resultText), &result); jsonErr != nil {
				t.Errorf("parsing result JSON %q: %v", resultText, jsonErr)
				return
			}

			icon := "✓"
			fail := false
			if tc.wantLoaded && !result.Loaded {
				icon = "✗"
				fail = true
			}
			if result.Loaded && result.CanvasBlank {
				icon = "✗"
				fail = true
			}
			t.Logf("%s loaded=%-5v blank=%-5v frames=%-4d error=%q",
				icon, result.Loaded, result.CanvasBlank, result.Frames, result.Error)

			if fail {
				if !result.Loaded {
					t.Errorf("expected loaded=true — runtime error: %s", result.Error)
				} else {
					t.Errorf("canvas is blank after %dms — paint invisible or shape off-screen", animDurationMs)
				}
			}
		})
	}
}

// generateTestFiles uses the builder to write .riv files into dir.
// Returns the list of file names created.
func generateTestFiles(t *testing.T, dir string) []string {
	t.Helper()

	type gen struct {
		name string
		fn   func() ([]byte, error)
	}
	gens := []gen{
		{"gen_fade_rect.riv", genFadeRect},
		{"gen_bounce_ball.riv", genBounceBall},
		{"gen_color_cycle.riv", genColorCycle},
		{"gen_gradient_ellipse.riv", genGradientEllipse},
		{"gen_multi_shape.riv", genMultiShape},
	}

	var names []string
	for _, g := range gens {
		data, err := g.fn()
		if err != nil {
			t.Errorf("generating %s: %v", g.name, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, g.name), data, 0644); err != nil {
			t.Errorf("writing %s: %v", g.name, err)
			continue
		}
		names = append(names, g.name)
	}
	return names
}

// launchBrowser finds or downloads Chromium and returns the CDP control URL.
// Calls t.Skip if no browser is obtainable.
func launchBrowser(t *testing.T) string {
	t.Helper()

	newLauncher := func(bin string) *launcher.Launcher {
		return launcher.New().
			Bin(bin).
			Headless(true).
			NoSandbox(true).
			Set(flags.Flag("disable-gpu")).
			Set(flags.Flag("disable-dev-shm-usage"))
	}

	// Check CHROME_BIN env var (explicit override — useful on Alpine/CI)
	if bin := os.Getenv("CHROME_BIN"); bin != "" {
		t.Logf("using CHROME_BIN: %s", bin)
		u, err := newLauncher(bin).Launch()
		if err != nil {
			t.Skipf("CHROME_BIN=%s launch failed: %v", bin, err)
		}
		return u
	}

	// Try system browser first
	if path, found := launcher.LookPath(); found {
		t.Logf("using system browser: %s", path)
		u, err := newLauncher(path).Launch()
		if err == nil {
			return u
		}
		t.Logf("system browser launch failed (%v) — falling back to Rod downloader", err)
	}

	// Auto-download Chromium
	t.Log("downloading Chromium via Rod (first run ~130MB, cached afterwards)…")
	path, err := launcher.NewBrowser().Get()
	if err != nil {
		t.Skipf("cannot obtain a browser binary: %v\n"+
			"To fix: install Chromium (apk add chromium / brew install chromium) or ensure google-chrome is in PATH.", err)
		return ""
	}
	t.Logf("downloaded to: %s", path)

	u, err := newLauncher(path).Launch()
	if err != nil {
		t.Skipf("browser launch failed: %v", err)
		return ""
	}
	return u
}

// findRepoRoot walks up from the test working directory until go.mod is found.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root (go.mod) not found — run from within the module")
		}
		dir = parent
	}
}

// ── Builder helpers (mirror cmd/examples/main.go) ───────────────────────────

func genFadeRect() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("FadeRect", 500, 500)
	rect := ab.Rectangle(150, 175, 200, 150).Fill(0xFFCC3333).Name("rect")
	ab.Animation("fadeIn",
		builder.WithDuration(30),
		builder.WithLoop(builder.Loop),
	).KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropOpacity, 30, 1.0, builder.Linear())
	return b.Bytes()
}

func genBounceBall() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("BounceBall", 400, 400)
	ball := ab.Ellipse(200, 100, 50, 50).Fill(0xFF4488FF).Name("ball")
	ab.Animation("bounce",
		builder.WithDuration(30),
		builder.WithFPS(30),
		builder.WithLoop(builder.PingPong),
	).KeyframeFloat(ball, builder.PropY, 0, 50.0, builder.Cubic(0.42, 0, 0, 1)).
		KeyframeFloat(ball, builder.PropY, 30, 320.0, builder.Cubic(0, 0, 0.58, 1))
	return b.Bytes()
}

func genColorCycle() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("ColorCycle", 400, 300)
	rect := ab.Rectangle(100, 75, 200, 150).Fill(0xFFFF0000).Name("rect")
	anim := ab.Animation("cycle",
		builder.WithDuration(60),
		builder.WithLoop(builder.Loop),
	)
	anim.KeyframeColor(rect, builder.PropColorValue, 0, 0xFFFF0000, builder.Hold())
	anim.KeyframeColor(rect, builder.PropColorValue, 20, 0xFF00CC00, builder.Hold())
	anim.KeyframeColor(rect, builder.PropColorValue, 40, 0xFF0044FF, builder.Hold())
	return b.Bytes()
}

func genGradientEllipse() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("GradientEllipse", 400, 400)
	ab.Ellipse(200, 200, 160, 160).FillGradient(0, -160, 0, 160,
		builder.GradientStop{Position: 0, Color: 0xFFFF4444},
		builder.GradientStop{Position: 1, Color: 0xFF4444FF},
	)
	return b.Bytes()
}

func genMultiShape() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("MultiShape", 600, 400)
	ab.Rectangle(80, 100, 120, 100).Fill(0xFFFF3333).Name("r1")
	ab.Rectangle(280, 100, 120, 100).Fill(0xFF33FF33).Name("r2")
	ab.Ellipse(480, 160, 80, 80).Fill(0xFF3333FF).Name("e1")
	return b.Bytes()
}
