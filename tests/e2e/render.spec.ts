/**
 * render.spec.ts — Feedback loop #2 / render gate
 *
 * For every .riv in examples/ and fromjson_*.riv:
 *   ✓ File loads without error (onLoadError never fires)
 *   ✓ Canvas has non-zero pixels after 1.5 s (not blank — something rendered)
 *
 * These are the structural smoke tests. They catch the class of bug where a
 * .riv parses correctly but the runtime renders nothing (blank canvas).
 */
import { test, expect } from '@playwright/test';

interface RiveResult {
  loaded: boolean;
  error: string;
  frames: number;
  canvasBlank: boolean;
}

// ── file inventory ────────────────────────────────────────────────────────────

interface FileEntry {
  name: string;
  path: string;
  sm?: string;    // state machine name (for SM-only files)
  anim?: string;  // animation name
}

const EXAMPLE_FILES: FileEntry[] = [
  { name: 'bounce_ball',       path: '/docs/preview/examples/bounce_ball.riv',       anim: 'bounce'      },
  { name: 'color_cycle',       path: '/docs/preview/examples/color_cycle.riv',       anim: 'colorCycle'  },
  { name: 'fade_rect',         path: '/docs/preview/examples/fade_rect.riv',         anim: 'fadeIn'      },
  { name: 'fade_rect_opacity', path: '/docs/preview/examples/fade_rect_opacity.riv'                     },
  { name: 'gradient_ellipse',  path: '/docs/preview/examples/gradient_ellipse.riv'                      },
  { name: 'minimal_static',    path: '/docs/preview/examples/minimal_static.riv'                        },
  { name: 'multi_shape',       path: '/docs/preview/examples/multi_shape.riv'                           },
  { name: 'reference',         path: '/docs/preview/examples/reference.riv',         sm: 'State Machine 1' },
  { name: 'spinning_square',   path: '/docs/preview/examples/spinning_square.riv',   anim: 'spin'        },
  { name: 'toggle_button',     path: '/docs/preview/examples/toggle_button.riv',     sm: 'ButtonSM'      },
  { name: 'interactive_button',path: '/docs/preview/examples/interactive_button.riv',sm: 'ButtonSM'      },
];

const FROMJSON_FILES: FileEntry[] = [
  { name: 'fromjson_spin',            path: '/docs/preview/fromjson_spin.riv',            anim: 'spin'   },
  { name: 'fromjson_dots',            path: '/docs/preview/fromjson_dots.riv',            anim: 'bounce' },
  { name: 'fromjson_gradient',        path: '/docs/preview/fromjson_gradient.riv',        anim: 'morph'  },
  { name: 'fromjson_color',           path: '/docs/preview/fromjson_color.riv'                           },
  { name: 'fromjson_easing',          path: '/docs/preview/fromjson_easing.riv'                         },
  { name: 'fromjson_autumn',          path: '/docs/preview/fromjson_autumn.riv'                         },
  { name: 'fromjson_card_shuffle',    path: '/docs/preview/fromjson_card_shuffle.riv'                   },
  { name: 'fromjson_draw_order',      path: '/docs/preview/fromjson_draw_order.riv'                     },
  { name: 'fromjson_draw_order_anim', path: '/docs/preview/fromjson_draw_order_anim.riv'                },
  { name: 'fromjson_toggle_button_sm',  path: '/docs/preview/fromjson_toggle_button_sm.riv',  sm: 'ToggleSM' },
  { name: 'fromjson_multistate_nav_sm', path: '/docs/preview/fromjson_multistate_nav_sm.riv', sm: 'NavSM'    },
  { name: 'fromjson_blend_slider_sm',   path: '/docs/preview/fromjson_blend_slider_sm.riv',   sm: 'BlendSM'  },
  { name: 'fromjson_star_path',  path: '/docs/preview/fromjson_star_path.riv',  anim: 'spin'  },
  { name: 'fromjson_bezier_leaf', path: '/docs/preview/fromjson_bezier_leaf.riv', anim: 'pulse' },
  { name: 'fromjson_path_morph', path: '/docs/preview/fromjson_path_morph.riv', anim: 'morph' },
  // fromjson_hello_world.riv contains an 80KB embedded TTF (codicon). The Rive
  // WASM runtime hangs processing this font in headless Chromium — neither
  // onLoad nor onLoadError fires within 30s. Text format correctness is fully
  // covered by Go unit tests (rive/builder/text_test.go +
  // rive/fromjson/text_fromjson_test.go, 26 tests). Skipping browser render
  // until a font-subsetting step is added to the build pipeline.
];

// ── helper ────────────────────────────────────────────────────────────────────

async function loadAndWait(page: import('@playwright/test').Page, entry: FileEntry): Promise<RiveResult> {
  const params = new URLSearchParams({ file: entry.path, duration: '1500' });
  if (entry.sm)   params.set('sm',   entry.sm);
  if (entry.anim) params.set('anim', entry.anim);

  await page.goto(`/tests/e2e/harness.html?${params}`);

  // Wait until harness writes result JSON into #result
  await page.waitForFunction(
    () => {
      const el = document.getElementById('result');
      return el != null && el.textContent != null && el.textContent.trim() !== '';
    },
    null,
    { timeout: 30_000 },
  );

  const text = await page.locator('#result').textContent();
  return JSON.parse(text ?? '{}') as RiveResult;
}

// ── tests ─────────────────────────────────────────────────────────────────────

test.describe('render: examples/', () => {
  for (const entry of EXAMPLE_FILES) {
    test(entry.name, async ({ page }) => {
      const result = await loadAndWait(page, entry);

      expect(result.loaded, `${entry.name}: loaded=false — error: ${result.error}`).toBe(true);
      expect(result.canvasBlank, `${entry.name}: canvas is blank after 1.5s`).toBe(false);
    });
  }
});

test.describe('render: fromjson/', () => {
  for (const entry of FROMJSON_FILES) {
    test(entry.name, async ({ page }) => {
      const result = await loadAndWait(page, entry);

      expect(result.loaded, `${entry.name}: loaded=false — error: ${result.error}`).toBe(true);
      expect(result.canvasBlank, `${entry.name}: canvas is blank after 1.5s`).toBe(false);
    });
  }
});
