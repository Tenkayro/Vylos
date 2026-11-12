import { execFileSync, spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const exePath = path.join(rootDir, 'FamilyControl.exe');

// Always ensure frontend/dist and FamilyControl.exe are built and up-to-date
execFileSync(process.execPath, [path.join(__dirname, 'build.mjs')], {
  cwd: rootDir,
  stdio: 'inherit',
});

if (!fs.existsSync(exePath)) {
  console.error('Error: FamilyControl.exe was not found after build.');
  process.exit(1);
}

const extraArgs = process.argv.slice(2);
console.log('Starting FamilyControl...');

const child = spawn(exePath, extraArgs, {
  cwd: rootDir,
  stdio: 'inherit',
});

process.on('SIGINT', () => {
  child.kill('SIGINT');
});
process.on('SIGTERM', () => {
  child.kill('SIGTERM');
});

child.on('exit', (code) => {
  process.exit(code ?? 0);
});
