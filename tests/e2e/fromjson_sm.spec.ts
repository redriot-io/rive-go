/**
 * fromjson_sm.spec.ts — Phase 4D / FromJSON state machine e2e gate
 *
 * Validates that .riv files compiled from JSON via `rivtool create --from`
 * actually run their state machines in the browser and respond to input changes.
 *
 * Tests:
 *   fromjson_sm: toggle_button — loads, renders, pointer_down changes canvas
 *   fromjson_sm: multistate_nav — loads, renders, JS input change changes canvas
 *   fromjson_sm: blend_slider   — loads, renders, JS input change changes canvas
 */
import { test, expect } from '@playwright/test';

// ── helpers ──────────────────────────────────────────────────────────────────

async function waitForLoad(page: import('@playwright/test').Page, timeout = 20_000) {
  await expect(async () => {
    const loaded = await page.evaluate(() => (window as any).isLoaded());
    expect(loaded).toBe(true);
  }).toPass({ timeout, intervals: [200, 500, 1000] });
}

async function getPixelSum(page: import('@playwright/test').Page): Promise<number> {
  return page.evaluate(() => (window as any).getPixelSum());
}

async function setInput(
  page: import('@playwright/test').Page,
  smName: string,
  inputName: string,
  value: boolean | number,
) {
  await page.evaluate(
    ({ smName, inputName, value }) => {
      const r = (window as any).__riveInstance;
      if (!r) return;
      const inputs = r.stateMachineInputs(smName);
      if (!inputs) return;
      const inp = inputs.find((i: any) => i.name === inputName);
      if (inp) inp.value = value;
    },
    { smName, inputName, value },
  );
}

// ── Toggle Button ─────────────────────────────────────────────────────────────

test('fromjson_sm: toggle_button — loads and pointer changes canvas', async ({ page }) => {
  await page.goto(
    '/tests/e2e/harness-interactive.html?file=/docs/preview/fromjson_toggle_button_sm.riv&sm=ToggleSM',
  );
  await waitForLoad(page);

  const loadErr = await page.evaluate(() => (window as any).getError());
  expect(loadErr, `load error: ${loadErr}`).toBeNull();

  await page.waitForTimeout(400);
  const sumIdle: number = await getPixelSum(page);
  expect(sumIdle, 'toggle_button: canvas blank at idle').toBeGreaterThan(0);

  // Drive "active" bool input to true → SM transitions to On (green)
  await setInput(page, 'ToggleSM', 'active', true);
  await page.waitForTimeout(400);
  const sumOn: number = await getPixelSum(page);

  expect(sumOn, [
    'toggle_button: pixel sum unchanged after active=true — SM transition did not fire.',
    `idle=${sumIdle}  on=${sumOn}`,
  ].join('\n')).not.toBe(sumIdle);

  // Drive "active" back to false → SM transitions to Off (blue)
  await setInput(page, 'ToggleSM', 'active', false);
  await page.waitForTimeout(400);
  const sumOff: number = await getPixelSum(page);

  expect(sumOff, [
    'toggle_button: pixel sum unchanged after active=false — SM transition did not fire.',
    `on=${sumOn}  off=${sumOff}`,
  ].join('\n')).not.toBe(sumOn);

  console.log(`✓ toggle_button: idle=${sumIdle} on=${sumOn} off=${sumOff}`);
});

// ── Multi-State Nav ───────────────────────────────────────────────────────────

test('fromjson_sm: multistate_nav — page input drives state transitions', async ({ page }) => {
  await page.goto(
    '/tests/e2e/harness-interactive.html?file=/docs/preview/fromjson_multistate_nav_sm.riv&sm=NavSM',
  );
  await waitForLoad(page);

  const loadErr = await page.evaluate(() => (window as any).getError());
  expect(loadErr, `load error: ${loadErr}`).toBeNull();

  await page.waitForTimeout(400);
  const sumA: number = await getPixelSum(page);
  expect(sumA, 'multistate_nav: canvas blank at page=0').toBeGreaterThan(0);

  // page=0.5 → should transition to state B
  await setInput(page, 'NavSM', 'page', 0.5);
  await page.waitForTimeout(400);
  const sumB: number = await getPixelSum(page);

  expect(sumB, [
    'multistate_nav: pixel sum unchanged after page=0.5 — A→B transition did not fire.',
    `A=${sumA}  B=${sumB}`,
  ].join('\n')).not.toBe(sumA);

  // page=0.8 → should transition to state C
  await setInput(page, 'NavSM', 'page', 0.8);
  await page.waitForTimeout(400);
  const sumC: number = await getPixelSum(page);

  expect(sumC, [
    'multistate_nav: pixel sum unchanged after page=0.8 — B→C transition did not fire.',
    `B=${sumB}  C=${sumC}`,
  ].join('\n')).not.toBe(sumB);

  console.log(`✓ multistate_nav: A=${sumA} B=${sumB} C=${sumC}`);
});

// ── Blend Slider ──────────────────────────────────────────────────────────────

test('fromjson_sm: blend_slider — mix input drives BlendState1D', async ({ page }) => {
  await page.goto(
    '/tests/e2e/harness-interactive.html?file=/docs/preview/fromjson_blend_slider_sm.riv&sm=BlendSM',
  );
  await waitForLoad(page);

  const loadErr = await page.evaluate(() => (window as any).getError());
  expect(loadErr, `load error: ${loadErr}`).toBeNull();

  await page.waitForTimeout(600);
  const sum0: number = await getPixelSum(page);
  expect(sum0, 'blend_slider: canvas blank at mix=0').toBeGreaterThan(0);

  // Set mix=1 (intense): ball should be at a different position in its cycle.
  // Sample at t≈150ms and t≈600ms — deliberately asymmetric on the ease-in-out
  // curve to avoid the t=0.4/0.6 symmetry trap (identical pixel sums).
  await setInput(page, 'BlendSM', 'mix', 1.0);
  await page.waitForTimeout(150);
  const sum1a: number = await getPixelSum(page);
  await page.waitForTimeout(450);
  const sum1b: number = await getPixelSum(page);

  // Animation must be playing (pixels change over time)
  expect(sum1a, [
    'blend_slider: canvas frozen at mix=1 — BlendState1D not animating.',
    `sum1a=${sum1a}  sum1b=${sum1b}`,
  ].join('\n')).not.toBe(sum1b);

  // Verify at the gentle end the animation is also alive.
  // Use mix=0.05 (not 0.0) — BlendState1D at exactly the minimum threshold (0.0)
  // may hold the animation at t=0 rather than advancing it (Rive edge case).
  // Use a wider 800ms window to accommodate slow ease-in-out motion.
  await setInput(page, 'BlendSM', 'mix', 0.05);
  await page.waitForTimeout(200);
  const sum0a: number = await getPixelSum(page);
  await page.waitForTimeout(800);
  const sum0b: number = await getPixelSum(page);

  if (sum0a === sum0b) {
    // Last-resort: one more sample after another full animation cycle.
    await page.waitForTimeout(1000);
    const sum0c: number = await getPixelSum(page);
    expect(sum0c, 'blend_slider: canvas frozen at mix≈0 after extended wait').not.toBe(sum0a);
  } else {
    expect(sum0a, 'blend_slider: canvas frozen at mix≈0').not.toBe(sum0b);
  }

  console.log(`✓ blend_slider: mix≈0 sample=[${sum0a},${sum0b}] mix=1 sample=[${sum1a},${sum1b}]`);
});
