#!/usr/bin/env node
// Validate all .riv files in a directory through the Rive WASM runtime.
// Usage: node validate-all.js [dir]
// Exit:  0=all pass  1=any failed
//
// Flags:
//   --no-render-assert  (accepted for compatibility, render is never attempted in
//                        headless Node — WebGL context is not available)
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
  const args = process.argv.slice(2);
  const dir  = args.find(a => !a.startsWith('--')) || 'docs/preview';

  // Init WASM once for all files
  let rive;
  try {
    const wasmBuf    = fs.readFileSync(path.join(pkgDir, 'rive.wasm'));
    const wasmBinary = wasmBuf.buffer.slice(wasmBuf.byteOffset, wasmBuf.byteOffset + wasmBuf.byteLength);
    const { default: RiveInit } = await import(pathToFileURL(path.join(pkgDir, 'canvas_advanced.mjs')).href);
    rive = await RiveInit({ wasmBinary });
  } catch(e) {
    console.error('FATAL: WASM init failed:', e?.message ?? e);
    process.exit(1);
  }

  let files;
  try {
    files = fs.readdirSync(dir).filter(f => f.endsWith('.riv')).sort();
  } catch(e) {
    console.error(`FATAL: cannot read directory ${dir}: ${e.message}`);
    process.exit(1);
  }

  if (files.length === 0) {
    console.error(`No .riv files found in ${dir}`);
    process.exit(1);
  }

  let pass = 0, fail = 0;
  for (const f of files) {
    const filePath = path.join(dir, f);
    try {
      const bytes = fs.readFileSync(filePath);
      const file  = await rive.load(new Uint8Array(bytes));

      if (!file || file.artboardCount() === 0) {
        console.log(`FAIL  ${f}  (null — asset decode or format failure)`);
        fail++;
        continue;
      }

      const ab = file.defaultArtboard();
      console.log(`PASS  ${f}  (artboard: "${ab.name}")`);
      pass++;
    } catch(e) {
      console.log(`FAIL  ${f}  (${e?.message ?? String(e)})`);
      fail++;
    }
  }

  console.log(`\n${pass} passed, ${fail} failed out of ${pass + fail} files`);
  process.exit(fail > 0 ? 1 : 0);
}

main().catch(e => {
  console.error('FATAL:', e?.message ?? e);
  process.exit(1);
});
