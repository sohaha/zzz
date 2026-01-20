#!/usr/bin/env node
const esbuild = require('esbuild');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const distDir = path.join(__dirname, 'dist');
const staticDir = path.join(__dirname, 'static');

// Ensure dist dir exists
if (!fs.existsSync(distDir)) {
  fs.mkdirSync(distDir, { recursive: true });
}

// Clean all old js/css files
const files = fs.readdirSync(distDir).filter(f => /\.(js|css)$/.test(f));
files.forEach(f => fs.unlinkSync(path.join(distDir, f)));

// Build with esbuild
esbuild.buildSync({
  entryPoints: ['src/main.ts'],
  bundle: true,
  minify: true,
  outdir: distDir,
  entryNames: '[name]',
});

// Calculate hash for the built files
function getFileHash(filePath) {
  const content = fs.readFileSync(filePath);
  return crypto.createHash('md5').update(content).digest('hex').slice(0, 8);
}

const jsHash = getFileHash(path.join(distDir, 'main.js'));
const cssPath = path.join(distDir, 'main.css');
const cssHash = fs.existsSync(cssPath) ? getFileHash(cssPath) : null;

// Rename files with hash
fs.renameSync(path.join(distDir, 'main.js'), path.join(distDir, `app.${jsHash}.js`));
if (cssHash) {
  fs.renameSync(cssPath, path.join(distDir, `app.${cssHash}.css`));
}

// Update index.html with hashed filenames
let html = fs.readFileSync(path.join(staticDir, 'index.html'), 'utf-8');
html = html.replace(/app(\.[a-f0-9]+)?\.js/g, `app.${jsHash}.js`);
html = html.replace(/app(\.[a-f0-9]+)?\.css/g, `app.${cssHash || jsHash}.css`);
fs.writeFileSync(path.join(distDir, 'index.html'), html);

console.log(`Built: app.${jsHash}.js, app.${cssHash || 'N/A'}.css`);
