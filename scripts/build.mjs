import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildMsi } from './build-msi.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const distDir = path.join(rootDir, 'dist');
fs.mkdirSync(distDir, { recursive: true });

// Remove legacy Vylos-Setup.msi if present
try {
  fs.rmSync(path.join(distDir, 'Vylos-Setup.msi'), { force: true });
} catch {}

const pkg = JSON.parse(fs.readFileSync(path.join(rootDir, 'package.json'), 'utf8'));
const version = (pkg.version || '1.0.0').replace(/^v/, '');

console.log('[1/4] Building Vylos Frontend assets...');
execFileSync(process.execPath, [path.join(rootDir, 'frontend', 'scripts', 'build.mjs')], {
  cwd: rootDir,
  stdio: 'inherit',
});

console.log(`[2/4] Compiling Vylos-Parent.exe & Vylos-Child.exe v${version} (from single codebase)...`);
const parentExe = path.join(distDir, 'Vylos-Parent.exe');
const childExe = path.join(distDir, 'Vylos-Child.exe');

execFileSync(
  'go',
  [
    'build',
    `-ldflags=-s -w -X familycontrol/backend/updater.AppVersion=${version} -X familycontrol/backend/family.BuildEdition=parent`,
    '-o',
    parentExe,
    '.',
  ],
  { cwd: rootDir, stdio: 'inherit' }
);

execFileSync(
  'go',
  [
    'build',
    `-ldflags=-s -w -X familycontrol/backend/updater.AppVersion=${version} -X familycontrol/backend/family.BuildEdition=child`,
    '-o',
    childExe,
    '.',
  ],
  { cwd: rootDir, stdio: 'inherit' }
);

// Also build Vylos.exe in root for local `npm run start` testing
execFileSync(
  'go',
  [
    'build',
    `-ldflags=-s -w -X familycontrol/backend/updater.AppVersion=${version} -X familycontrol/backend/family.BuildEdition=unified`,
    '-o',
    path.join(rootDir, 'Vylos.exe'),
    '.',
  ],
  { cwd: rootDir, stdio: 'inherit' }
);

console.log('[3/4] Packaging dist/Vylos-Parent.msi...');
buildMsi({ version, edition: 'parent', sourceExePath: parentExe });

console.log('[4/4] Packaging dist/Vylos-Child.msi...');
buildMsi({ version, edition: 'child', sourceExePath: childExe });

console.log('Build complete: dist/Vylos-Parent.msi and dist/Vylos-Child.msi are ready.');
