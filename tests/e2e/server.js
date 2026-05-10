// Minimal static HTTP server — serves the repo root on PORT 5173.
// Used by playwright.config.ts webServer. No external dependencies.
const http = require('http');
const fs   = require('fs');
const path = require('path');

const PORT = parseInt(process.env.PORT || '5173', 10);
const ROOT = path.resolve(__dirname, '../..');

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js':   'application/javascript',
  '.css':  'text/css',
  '.riv':  'application/octet-stream',
  '.json': 'application/json',
  '.ts':   'text/plain',
  '.png':  'image/png',
  '.svg':  'image/svg+xml',
};

const server = http.createServer((req, res) => {
  const urlPath = req.url.split('?')[0];
  const filePath = path.normalize(path.join(ROOT, decodeURIComponent(urlPath)));

  // Prevent path traversal
  if (!filePath.startsWith(ROOT)) {
    res.writeHead(403);
    res.end('Forbidden');
    return;
  }

  fs.readFile(filePath, (err, data) => {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end(`Not found: ${urlPath}`);
      return;
    }
    const ext = path.extname(filePath).toLowerCase();
    res.writeHead(200, {
      'Content-Type': MIME[ext] || 'application/octet-stream',
      'Cache-Control': 'no-cache',
    });
    res.end(data);
  });
});

server.listen(PORT, () => {
  console.log(`rive-go e2e server → http://127.0.0.1:${PORT}  (root: ${ROOT})`);
});
