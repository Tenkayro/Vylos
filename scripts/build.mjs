import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildMsi } from './build-msi.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const pkg = JSON.parse(fs.readFileSync(path.join(rootDir, 'package.json'), 'utf8'));
const version = (pkg.version || '1.0.0').replace(/^v/, '');

console.log('[1/3] Building Vylos Frontend assets...');
execFileSync(process.execPath, [path.join(rootDir, 'frontend', 'scripts', 'build.mjs')], {
  cwd: rootDir,
  stdio: 'inherit',
});

console.log(`[2/3] Compiling Vylos.exe v${version} (Unified Parent & Child Application)...`);
execFileSync(
  'go',
  [
    'build',
    `-ldflags=-s -w -X familycontrol/backend/updater.AppVersion=${version}`,
    '-o',
    'Vylos.exe',
    '.',
  ],
  {
    cwd: rootDir,
    stdio: 'inherit',
  }
);

console.log('[3/3] Packaging unified Windows Installer (dist/Vylos-Setup.msi)...');
buildMsi({ version, sourceExePath: path.join(rootDir, 'Vylos.exe') });

console.log('Build complete: Vylos.exe and dist/Vylos-Setup.msi are ready.');
