import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

function formatMinutes(totalMinutes) {
  if (!totalMinutes || totalMinutes <= 0) return '0m';
  const h = Math.floor(totalMinutes / 60);
  const m = totalMinutes % 60;
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}h`;
  return `${h}h ${m}m`;
}

function validateDomainInput(raw) {
  const cleaned = String(raw || '')
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, '')
    .split('/')[0];
  if (!cleaned) return null;
  const re = /^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$/;
  return re.test(cleaned) ? cleaned : null;
}

test('formatMinutes formats screen-time durations accurately', () => {
  assert.equal(formatMinutes(0), '0m');
  assert.equal(formatMinutes(45), '45m');
  assert.equal(formatMinutes(60), '1h');
  assert.equal(formatMinutes(135), '2h 15m');
});

test('validateDomainInput normalizes and validates domains for DNS/hosts rules', () => {
  assert.equal(validateDomainInput('https://www.TikTok.com/discover'), 'www.tiktok.com');
  assert.equal(validateDomainInput('khanacademy.org'), 'khanacademy.org');
  assert.equal(validateDomainInput('not a domain'), null);
  assert.equal(validateDomainInput('http://-invalid.com'), null);
});

test('Svelte pages and dark-theme CSS palette match specification', () => {
  const css = fs.readFileSync(path.join(rootDir, 'src', 'styles', 'app.css'), 'utf8');
  for (const hex of ['#0C1017', '#101620', '#141C27', '#273344', '#E8EDF5', '#8B98AA', '#DCE9FF', '#59C99A', '#E5B96B', '#E47F7F']) {
    assert.ok(css.includes(hex), `Expected CSS palette to contain ${hex}`);
  }

  const requiredPages = [
    'OverviewPage.svelte',
    'DevicesPage.svelte',
    'ActivityPage.svelte',
    'ScreenTimePage.svelte',
    'ApplicationsPage.svelte',
    'WebsitesPage.svelte',
    'ProtectionPage.svelte',
    'SettingsPage.svelte',
    'ChildModePage.svelte',
  ];
  for (const page of requiredPages) {
    assert.ok(fs.existsSync(path.join(rootDir, 'src', 'pages', page)), `Missing page ${page}`);
  }
});
