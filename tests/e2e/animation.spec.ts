/**
 * animation.spec.ts — Feedback loop #2 / animation gate
 *
 * For each animated .riv:
 *   ✓ File loads (inherited from render tests)
 *   ✓ Canvas pixel sum at t=0 differs from pixel sum at t+600ms
 *     (proves the animation is actually playing, not frozen)
 *
 * Uses window.getPixelSum() exposed by harness.html. A frozen animation
 * (wrong interpTypeCode, Speed=0, WorkEnd=0) will produce identical sums.
 */
import { test, expect } from '@playwright/test';

interface AnimFile {
  name: string;
  path: string;
  sm?:  string;
  anim?: string;
}

const ANIM_FILES: AnimFile[] = [
  { name: 'bounce_ball',     path: '/docs/preview/examples/bounce_ball.riv',     anim: 'bounce'     },
  { name: 'color_cycle',     path: '/docs/preview/examples/color_cycle.riv',     anim: 'colorCycle' },
  { name: 'fade_rect',       path: '/docs/preview/examples/fade_rect.riv',       anim: 'fadeIn'     },
  { name: 'spinning_square', path: '/docs/preview/examples/spinning_square.riv', anim: 'spin'       },
  { name: 'fromjson_spin',   path: '/docs/preview/fromjson_spin.riv',            anim: 'spin'       },
  { name: 'fromjson_dots',   path: '/docs/preview/fromjson_dots.riv',            anim: 'bounce'     },
  { name: 'fromjson_color',  path: '/docs/preview/fromjson_color.riv'                              },
  { name: 'fromjson_autumn', path: '/docs/preview/fromjson_autumn.riv'                             },
];

for (const entry of ANIM_FILES) {
  test(`animation: ${entry.name}`, async ({ page }) => {
    const params = new URLSearchParams({ file: entry.path, duration: '500' });
    if (entry.sm)   params.set('sm',   entry.sm);
    if (entry.anim) params.set('anim', entry.anim);

    await page.goto(`/tests/e2e/harness.html?${params}`);

    // Wait for the file to load (#result appears after duration ms)
    await page.waitForFunction(
      () => {
        const el = document.getElementById('result');
        return el != null && el.textContent != null && el.textContent.trim() !== '';
      },
      null,
      { timeout: 25_000 },
    );

    const text = await page.locator('#result').textContent();
    const result = JSON.parse(text ?? '{}');
    expect(result.loaded, `${entry.name}: file did not load — ${result.error}`).toBe(true);

    // Sample pixel sum at t≈500ms (harness just settled)
    const sum1: number = await page.evaluate(() => (window as any).getPixelSum());
    expect(sum1, `${entry.name}: getPixelSum() returned -1 (canvas read failed)`).toBeGreaterThan(0);

    // Wait for the animation to advance ~600ms more
    await page.waitForTimeout(600);

    // Sample again
    const sum2: number = await page.evaluate(() => (window as any).getPixelSum());

    // Assert pixels changed — animation is NOT frozen
    expect(sum1, `${entry.name}: pixel sum unchanged after 600ms — animation may be frozen`).not.toBe(sum2);
  });
}
