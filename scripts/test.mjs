import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

console.log('=== Running Frontend Tests ===');
execFileSync(process.execPath, ['--test', path.join(rootDir, 'frontend', 'scripts', 'test.mjs')], {
  cwd: rootDir,
  stdio: 'inherit',
});

console.log('\n=== Running Go Backend & Integration Tests ===');
execFileSync('go', ['test', '-v', './...'], {
  cwd: rootDir,
  stdio: 'inherit',
});
