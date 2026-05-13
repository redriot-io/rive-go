#!/usr/bin/env node
// Validate .riv files through the Rive WASM runtime (Node.js, no browser).
// Usage: node scripts/validate_wasm.mjs [dir] [single_file]
import { readFileSync, readdirSync } from 'fs';
import { join, resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dir = dirname(fileURLToPath(import.meta.url));
const pkgDir = resolve(__dir, '../node_modules/@rive-app/canvas-advanced');

// ── Minimal DOM/WebGL stubs for Emscripten ───────────────────────────────────
const glProxy = new Proxy({}, { get(_, p) {
  if (p === 'getExtension') return () => null;
  if (p === 'getParameter')  return () => 0;
  return () => null;
}});
const mockCanvas = {
  width: 1, height: 1, style: { width:'', height:'' },
  getContext: (type) => type === '2d' ? {
    drawImage:()=>{}, fillRect:()=>{},
    getImageData:()=>({ data: new Uint8ClampedArray(4) }),
    createImageData:(w,h)=>({ data:new Uint8ClampedArray(w*h*4), width:w, height:h }),
    putImageData:()=>{},
  } : glProxy,
  addEventListener:()=>{}, removeEventListener:()=>{},
  getBoundingClientRect:()=>({ left:0, top:0, width:1, height:1 }),
};
if (typeof document === 'undefined') {
  global.document = {
    createElement:(t)=>t==='canvas'?mockCanvas:{style:{}},
    createElementNS:(_,t)=>global.document.createElement(t),
    getElementById:()=>null, querySelector:()=>null,
    body:{ appendChild:()=>{}, removeChild:()=>{} },
  };
}
if (typeof window   === 'undefined') global.window = global;
if (typeof self     === 'undefined') global.self   = global;
if (typeof HTMLCanvasElement === 'undefined') global.HTMLCanvasElement = class {};
if (typeof OffscreenCanvas   === 'undefined') {
  global.OffscreenCanvas = class {
    constructor(w,h){ this.width=w; this.height=h; }
    getContext = mockCanvas.getContext;
    addEventListener = ()=>{};
  };
}
// Image element — needed so embedded-image PNGs can be decoded during rive.load().
if (typeof Image === 'undefined') {
  global.Image = class {
    constructor(){ this.width=8; this.height=8; this.naturalWidth=8; this.naturalHeight=8; }
    set src(_){ setImmediate(()=>{ if(this.onload) this.onload({ target:this }); }); }
  };
}

// ── Init WASM (provide binary directly to avoid Node fetch URL issues) ────────
let rive;
try {
  const wasmBuf = readFileSync(join(pkgDir, 'rive.wasm'));
  const wasmBinary = wasmBuf.buffer.slice(wasmBuf.byteOffset, wasmBuf.byteOffset+wasmBuf.byteLength);
  const { default: RiveInit } = await import('@rive-app/canvas-advanced');
  rive = await RiveInit({ wasmBinary });
} catch(e) {
  console.error('FATAL: WASM init failed:', e?.message ?? e);
  process.exit(2);
}

// ── Validate ─────────────────────────────────────────────────────────────────
const dir    = process.argv[2] || 'docs/preview';
const single = process.argv[3];
const files  = single
  ? [single]
  : readdirSync(dir).filter(f=>f.endsWith('.riv')).sort();

let pass=0, fail=0;
for (const f of files) {
  const path = f.includes('/') ? f : join(dir, f);
  try {
    const bytes = readFileSync(path);
    const file  = await rive.load(new Uint8Array(bytes));
    if (file && file.artboardCount() > 0) {
      const ab = file.defaultArtboard();
      console.log(`PASS  ${f}  (artboard: "${ab.name}")`);
      pass++;
    } else {
      console.log(`FAIL  ${f}  (null — embedded-asset decode failure)`);
      fail++;
    }
  } catch(e) {
    console.log(`FAIL  ${f}  (${e?.message??String(e)})`);
    fail++;
  }
}
console.log(`\n${pass} passed, ${fail} failed out of ${pass+fail} files`);
process.exit(fail > 0 ? 1 : 0);
