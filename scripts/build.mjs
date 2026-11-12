import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

console.log('[1/2] Building Frontend assets...');
execFileSync(process.execPath, [path.join(rootDir, 'frontend', 'scripts', 'build.mjs')], {
  cwd: rootDir,
  stdio: 'inherit',
});

console.log('[2/2] Compiling FamilyControl.exe (Go backend + embedded UI)...');
execFileSync('go', ['build', '-ldflags=-s -w', '-o', 'FamilyControl.exe', '.'], {
  cwd: rootDir,
  stdio: 'inherit',
});

console.log('Build complete: FamilyControl.exe is ready.');
