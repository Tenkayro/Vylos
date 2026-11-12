import * as p from '@clack/prompts';
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { buildMsi } from './build-msi.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const GITHUB_REPO = 'Tenkayro/Vylos';
const GITHUB_REPO_URL = `https://github.com/${GITHUB_REPO}`;

function parseArgs(argv) {
  const opts = {
    tag: '',
    notes: '',
    yes: false,
    dryRun: false,
    allowDirty: false,
  };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--tag' && argv[i + 1]) opts.tag = argv[++i];
    else if (a.startsWith('--tag=')) opts.tag = a.slice('--tag='.length);
    else if (a === '--notes' && argv[i + 1]) opts.notes = argv[++i];
    else if (a.startsWith('--notes=')) opts.notes = a.slice('--notes='.length);
    else if (a === '--yes' || a === '-y') opts.yes = true;
    else if (a === '--dry-run') opts.dryRun = true;
    else if (a === '--allow-dirty') opts.allowDirty = true;
  }
  return opts;
}

function parseSemver(raw) {
  const clean = String(raw || '').trim().replace(/^v/i, '');
  const m = clean.match(/^(\d+)\.(\d+)\.(\d+)$/);
  if (!m) return null;
  return { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]) };
}

function bumpSemver(ver, type) {
  const s = parseSemver(ver) || { major: 1, minor: 0, patch: 0 };
  if (type === 'major') return `${s.major + 1}.0.0`;
  if (type === 'minor') return `${s.major}.${s.minor + 1}.0`;
  return `${s.major}.${s.minor}.${s.patch + 1}`;
}

function sha256File(filePath) {
  const buf = fs.readFileSync(filePath);
  return crypto.createHash('sha256').update(buf).digest('hex');
}

function sha512Base64File(filePath) {
  const buf = fs.readFileSync(filePath);
  return crypto.createHash('sha512').update(buf).digest('base64');
}

function updateVersionFiles(newVersion) {
  const rootPkgPath = path.join(rootDir, 'package.json');
  const rootPkg = JSON.parse(fs.readFileSync(rootPkgPath, 'utf8'));
  rootPkg.version = newVersion;
  fs.writeFileSync(rootPkgPath, JSON.stringify(rootPkg, null, 2) + '\n', 'utf8');

  const fePkgPath = path.join(rootDir, 'frontend', 'package.json');
  if (fs.existsSync(fePkgPath)) {
    const fePkg = JSON.parse(fs.readFileSync(fePkgPath, 'utf8'));
    fePkg.version = newVersion;
    fs.writeFileSync(fePkgPath, JSON.stringify(fePkg, null, 2) + '\n', 'utf8');
  }

  const wailsPath = path.join(rootDir, 'wails.json');
  if (fs.existsSync(wailsPath)) {
    const wailsCfg = JSON.parse(fs.readFileSync(wailsPath, 'utf8'));
    if (wailsCfg.info) wailsCfg.info.productVersion = newVersion;
    fs.writeFileSync(wailsPath, JSON.stringify(wailsCfg, null, 2) + '\n', 'utf8');
  }

  const wxsPath = path.join(rootDir, 'installer', 'Vylos.wxs');
  if (fs.existsSync(wxsPath)) {
    let wxs = fs.readFileSync(wxsPath, 'utf8');
    wxs = wxs.replace(
      /<\?define ProductVersion = "[^"]+" \?>/,
      `<?define ProductVersion = "${newVersion}" ?>`
    );
    fs.writeFileSync(wxsPath, wxs, 'utf8');
  }
}

function findGhCli() {
  const candidates = [
    'C:\\Program Files\\GitHub CLI\\gh.exe',
    'C:\\Program Files (x86)\\GitHub CLI\\gh.exe',
  ];
  for (const c of candidates) {
    if (fs.existsSync(c)) return c;
  }
  return 'gh';
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  p.intro('Vylos Family Control — Release & Dual MSI Builder (Parent + Child)');

  const rootPkg = JSON.parse(fs.readFileSync(path.join(rootDir, 'package.json'), 'utf8'));
  const currentVersion = (rootPkg.version || '1.0.0').replace(/^v/, '');

  let nextVersion = opts.tag ? opts.tag.replace(/^v/, '') : '';

  if (!nextVersion) {
    const patchVer = bumpSemver(currentVersion, 'patch');
    const minorVer = bumpSemver(currentVersion, 'minor');
    const majorVer = bumpSemver(currentVersion, 'major');

    const bumpChoice = await p.select({
      message: `Current version is v${currentVersion}. Select release version bump:`,
      options: [
        { value: patchVer, label: `Patch  -> v${patchVer}`, hint: 'Bug fixes & maintenance' },
        { value: minorVer, label: `Minor  -> v${minorVer}`, hint: 'New features (backwards-compatible)' },
        { value: majorVer, label: `Major  -> v${majorVer}`, hint: 'Major release / architecture upgrade' },
        { value: 'keep', label: `Keep   -> v${currentVersion}`, hint: 'Re-build current version' },
        { value: 'custom', label: 'Custom version...', hint: 'Enter manually' },
      ],
    });

    if (p.isCancel(bumpChoice)) {
      p.cancel('Release cancelled.');
      process.exit(0);
    }

    if (bumpChoice === 'custom') {
      const customInput = await p.text({
        message: 'Enter custom semantic version (e.g. 1.2.0):',
        placeholder: patchVer,
        validate(val) {
          if (!parseSemver(val)) return 'Please enter a valid semantic version (X.Y.Z)';
        },
      });
      if (p.isCancel(customInput)) {
        p.cancel('Release cancelled.');
        process.exit(0);
      }
      nextVersion = String(customInput).trim().replace(/^v/, '');
    } else if (bumpChoice === 'keep') {
      nextVersion = currentVersion;
    } else {
      nextVersion = String(bumpChoice);
    }
  }

  const tag = `v${nextVersion}`;
  let releaseNotes = opts.notes;
  if (!releaseNotes && !opts.yes) {
    const notesInput = await p.text({
      message: 'Release notes (included in latest.json & GitHub Release):',
      placeholder: `Vylos Family Control ${tag} — Vylos-Parent.msi & Vylos-Child.msi`,
      defaultValue: `Vylos Family Control ${tag} — Vylos-Parent.msi & Vylos-Child.msi`,
    });
    if (p.isCancel(notesInput)) {
      p.cancel('Release cancelled.');
      process.exit(0);
    }
    releaseNotes = String(notesInput).trim();
  }
  if (!releaseNotes) {
    releaseNotes = `Vylos Family Control ${tag} — Vylos-Parent.msi & Vylos-Child.msi`;
  }

  p.note(
    [
      `Brand:        Vylos Family Control`,
      `Version:      v${currentVersion} -> ${tag}`,
      `Repository:   ${GITHUB_REPO_URL}`,
      `MSI Editions: dist/Vylos-Parent.msi (Parent Dashboard — blocked on Child PCs)`,
      `              dist/Vylos-Child.msi  (Child Agent — locked to Child Mode)`,
      `Updater:      dist/latest.json (Single manifest for Parent & Child Updater)`,
      `Mode:         ${opts.dryRun ? 'DRY RUN (local build only)' : 'Build + Publish GitHub Release'}`,
    ].join('\n'),
    'Release Plan'
  );

  if (!opts.yes) {
    const confirm = await p.confirm({
      message: `Proceed with building ${tag} and ${opts.dryRun ? 'generating local release artifacts' : 'publishing to GitHub Releases'}?`,
      initialValue: true,
    });
    if (p.isCancel(confirm) || !confirm) {
      p.cancel('Release cancelled.');
      process.exit(0);
    }
  }

  const s = p.spinner();

  // Step 1: Update version metadata
  s.start(`Updating version metadata to ${tag}...`);
  updateVersionFiles(nextVersion);
  s.stop(`Updated version metadata to ${tag}.`);

  // Step 2: Build Frontend
  s.start('Building Vylos frontend bundle...');
  execFileSync(process.execPath, [path.join(rootDir, 'frontend', 'scripts', 'build.mjs')], {
    cwd: rootDir,
    stdio: 'pipe',
  });
  s.stop('Built frontend assets.');

  // Step 3: Build Parent & Child editions from the single codebase
  const distDir = path.join(rootDir, 'dist');
  fs.mkdirSync(distDir, { recursive: true });
  try {
    fs.rmSync(path.join(distDir, 'Vylos-Setup.msi'), { force: true });
    fs.rmSync(path.join(distDir, 'Vylos-windows-amd64.exe'), { force: true });
    fs.rmSync(path.join(distDir, 'Vylos-windows-arm64.exe'), { force: true });
  } catch {}

  const parentExe = path.join(distDir, 'Vylos-Parent.exe');
  const childExe = path.join(distDir, 'Vylos-Child.exe');

  s.start('Compiling Vylos-Parent.exe & Vylos-Child.exe...');
  execFileSync(
    'go',
    [
      'build',
      `-ldflags=-s -w -X familycontrol/backend/updater.AppVersion=${nextVersion} -X familycontrol/backend/family.BuildEdition=parent`,
      '-o',
      parentExe,
      '.',
    ],
    {
      cwd: rootDir,
      env: { ...process.env, GOOS: 'windows', GOARCH: 'amd64' },
      stdio: 'pipe',
    }
  );
  execFileSync(
    'go',
    [
      'build',
      `-ldflags=-s -w -X familycontrol/backend/updater.AppVersion=${nextVersion} -X familycontrol/backend/family.BuildEdition=child`,
      '-o',
      childExe,
      '.',
    ],
    {
      cwd: rootDir,
      env: { ...process.env, GOOS: 'windows', GOARCH: 'amd64' },
      stdio: 'pipe',
    }
  );
  s.stop('Compiled Vylos-Parent.exe and Vylos-Child.exe.');

  // Step 4: Build both MSI Installers (dist/Vylos-Parent.msi & dist/Vylos-Child.msi)
  s.start('Building dist/Vylos-Parent.msi and dist/Vylos-Child.msi...');
  const parentMsiPath = path.join(distDir, 'Vylos-Parent.msi');
  const childMsiPath = path.join(distDir, 'Vylos-Child.msi');
  buildMsi({
    version: nextVersion,
    edition: 'parent',
    sourceExePath: parentExe,
    outputMsiPath: parentMsiPath,
  });
  buildMsi({
    version: nextVersion,
    edition: 'child',
    sourceExePath: childExe,
    outputMsiPath: childMsiPath,
  });
  s.stop('Created dist/Vylos-Parent.msi and dist/Vylos-Child.msi.');

  // Step 5: Generate single latest.json for the Auto-Updater (covers both Parent & Child editions)
  s.start('Generating latest.json updater manifest...');
  try {
    fs.rmSync(path.join(distDir, 'latest.yml'), { force: true });
  } catch {}
  const pubDate = new Date().toISOString();
  const parentMsiSha256 = sha256File(parentMsiPath);
  const parentMsiSize = fs.statSync(parentMsiPath).size;

  const childMsiSha256 = sha256File(childMsiPath);
  const childMsiSize = fs.statSync(childMsiPath).size;

  const parentMsiUrl = `${GITHUB_REPO_URL}/releases/download/${tag}/Vylos-Parent.msi`;
  const childMsiUrl = `${GITHUB_REPO_URL}/releases/download/${tag}/Vylos-Child.msi`;

  const latestJson = {
    version: nextVersion,
    tag,
    notes: releaseNotes,
    pub_date: pubDate,
    url: parentMsiUrl,
    sha256: parentMsiSha256,
    platforms: {
      'windows-x86_64': {
        url: parentMsiUrl,
        sha256: parentMsiSha256,
        size: parentMsiSize,
      },
      'windows-x86_64-parent': {
        url: parentMsiUrl,
        sha256: parentMsiSha256,
        size: parentMsiSize,
      },
      'windows-x86_64-child': {
        url: childMsiUrl,
        sha256: childMsiSha256,
        size: childMsiSize,
      },
    },
  };

  const latestJsonPath = path.join(distDir, 'latest.json');
  fs.writeFileSync(latestJsonPath, JSON.stringify(latestJson, null, 2) + '\n', 'utf8');
  s.stop('Generated dist/latest.json.');

  if (opts.dryRun) {
    p.outro(`Dry run complete! Built Vylos-Parent.msi, Vylos-Child.msi, and latest.json.`);
    return;
  }

  // Step 6: Git commit, tag & GitHub Release upload
  s.start(`Publishing ${tag} to GitHub (${GITHUB_REPO})...`);
  try {
    try {
      execFileSync('git', ['rev-parse', '--is-inside-work-tree'], { cwd: rootDir, stdio: 'pipe' });
    } catch {
      execFileSync('git', ['init'], { cwd: rootDir, stdio: 'pipe' });
    }

    try {
      const remotes = execFileSync('git', ['remote'], { cwd: rootDir, encoding: 'utf8' });
      if (!remotes.split(/\r?\n/).includes('origin')) {
        execFileSync('git', ['remote', 'add', 'origin', `${GITHUB_REPO_URL}.git`], { cwd: rootDir, stdio: 'pipe' });
      }
    } catch {}

    execFileSync('git', ['add', 'package.json', 'frontend/package.json', 'wails.json', 'installer/Vylos.wxs'], {
      cwd: rootDir,
      stdio: 'pipe',
    });

    try {
      execFileSync('git', ['commit', '-m', `chore(release): ${tag}`], { cwd: rootDir, stdio: 'pipe' });
    } catch {}

    try {
      execFileSync('git', ['tag', '-f', tag], { cwd: rootDir, stdio: 'pipe' });
    } catch {}

    try {
      execFileSync('git', ['push', 'origin', 'HEAD', '--tags', '--force'], { cwd: rootDir, stdio: 'pipe' });
    } catch {}

    const gh = findGhCli();
    const releaseAssets = [parentMsiPath, childMsiPath, latestJsonPath];

    let exists = false;
    try {
      execFileSync(gh, ['release', 'view', tag, '--repo', GITHUB_REPO], { cwd: rootDir, stdio: 'pipe' });
      exists = true;
    } catch {
      exists = false;
    }

    if (exists) {
      execFileSync(
        gh,
        ['release', 'upload', tag, ...releaseAssets, '--clobber', '--repo', GITHUB_REPO],
        { cwd: rootDir, stdio: 'inherit' }
      );
    } else {
      execFileSync(
        gh,
        [
          'release',
          'create',
          tag,
          ...releaseAssets,
          '--repo',
          GITHUB_REPO,
          '--title',
          `Vylos Family Control ${tag}`,
          '--notes',
          releaseNotes,
          '--latest',
        ],
        { cwd: rootDir, stdio: 'inherit' }
      );
    }
    s.stop(`Published ${tag} to ${GITHUB_REPO_URL}/releases/tag/${tag}`);
  } catch (err) {
    s.stop(`Local MSIs & manifests built; GitHub upload encountered an error: ${err.message}`);
    process.exitCode = 1;
    return;
  }

  p.outro(`Successfully built and released Vylos Family Control ${tag} (Vylos-Parent.msi & Vylos-Child.msi)!`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
