#!/usr/bin/env node
// Validate a single .riv file through the Rive WASM runtime.
// Usage: node validate.js <file.riv> [--expect-render]
// Exit:  0=pass  1=load error  2=render fail  3=harness error
'use strict';

const fs   = require('fs');
const path = require('path');
const { pathToFileURL } = require('url');

const pkgDir = path.resolve(__dirname, 'node_modules/@rive-app/canvas-advanced');

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
if (typeof Image === 'undefined') {
  global.Image = class {
    constructor(){ this.width=8; this.height=8; this.naturalWidth=8; this.naturalHeight=8; }
    set src(_){ setImmediate(()=>{ if(this.onload) this.onload({ target:this }); }); }
  };
}

async function main() {
  const args        = process.argv.slice(2);
  const filePath    = args.find(a => !a.startsWith('--'));
  const expectRender = args.includes('--expect-render');

  if (!filePath) {
    console.error('Usage: node validate.js <file.riv> [--expect-render]');
    process.exit(3);
  }

  // Init WASM
  let rive;
  try {
    const wasmBuf    = fs.readFileSync(path.join(pkgDir, 'rive.wasm'));
    const wasmBinary = wasmBuf.buffer.slice(wasmBuf.byteOffset, wasmBuf.byteOffset + wasmBuf.byteLength);
    const { default: RiveInit } = await import(pathToFileURL(path.join(pkgDir, 'canvas_advanced.mjs')).href);
    rive = await RiveInit({ wasmBinary });
  } catch(e) {
    console.error('FATAL: WASM init failed:', e?.message ?? e);
    process.exit(3);
  }

  // Read file
  let bytes;
  try {
    bytes = fs.readFileSync(filePath);
  } catch(e) {
    console.error(`FATAL: cannot read ${filePath}: ${e.message}`);
    process.exit(3);
  }

  // Load .riv
  let file;
  try {
    file = await rive.load(new Uint8Array(bytes));
  } catch(e) {
    console.error(`FAIL  ${filePath}  (exception: ${e?.message ?? e})`);
    process.exit(1);
  }

  if (!file || file.artboardCount() === 0) {
    console.error(`FAIL  ${filePath}  (null — asset decode or format failure)`);
    process.exit(1);
  }

  const ab = file.defaultArtboard();

  if (expectRender) {
    try {
      const renderer = rive.makeRenderer(mockCanvas);
      renderer.save();
      ab.advance(0);
      ab.draw(renderer);
      renderer.restore();
    } catch(e) {
      console.error(`FAIL  ${filePath}  (render error: ${e?.message ?? e})`);
      process.exit(2);
    }
  }

  console.log(`PASS  ${filePath}  (artboard: "${ab.name}")`);
  process.exit(0);
}

main().catch(e => {
  console.error('FATAL:', e?.message ?? e);
  process.exit(3);
});
