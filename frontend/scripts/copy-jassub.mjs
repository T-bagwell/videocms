// Copies the jassub (libass WASM) assets from node_modules into public/jassub
// so Vite can serve them as static files in dev and in the production build.
import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const src = join(root, '..', 'node_modules', 'jassub', 'dist');
const dst = join(root, '..', 'public', 'jassub');
mkdirSync(dst, { recursive: true });

const files = [
  ['wasm/jassub-worker.js', 'jassub-worker.js'],
  ['wasm/jassub-worker.wasm', 'jassub-worker.wasm'],
  ['wasm/jassub-worker-modern.wasm', 'jassub-worker-modern.wasm'],
  ['default.woff2', 'default.woff2'],
];
for (const [from, to] of files) {
  copyFileSync(join(src, from), join(dst, to));
}
console.log('jassub assets copied to public/jassub');
