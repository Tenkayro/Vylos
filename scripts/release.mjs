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
  // 1. Root package.json
  const rootPkgPath = path.join(rootDir, 'package.json');
  const rootPkg = JSON.parse(fs.readFileSync(rootPkgPath, 'utf8'));
  rootPkg.version = newVersion;
  fs.writeFileSync(rootPkgPath, JSON.stringify(rootPkg, null, 2) + '\n', 'utf8');

  // 2. Frontend package.json
  const fePkgPath = path.join(rootDir, 'frontend', 'package.json');
  if (fs.existsSync(fePkgPath)) {
    const fePkg = JSON.parse(fs.readFileSync(fePkgPath, 'utf8'));
    fePkg.version = newVersion;
    fs.writeFileSync(fePkgPath, JSON.stringify(fePkg, null, 2) + '\n', 'utf8');
  }

  // 3. wails.json
  const wailsPath = path.join(rootDir, 'wails.json');
  if (fs.existsSync(wailsPath)) {
    const wailsCfg = JSON.parse(fs.readFileSync(wailsPath, 'utf8'));
    if (wailsCfg.info) wailsCfg.info.productVersion = newVersion;
    fs.writeFileSync(wailsPath, JSON.stringify(wailsCfg, null, 2) + '\n', 'utf8');
  }

  // 4. installer/Vylos.wxs default ProductVersion
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
  p.intro('Vylos Family Control — Release & MSI Builder');

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
      message: 'Release notes (included in latest.json / latest.yml & GitHub Release):',
      placeholder: `Vylos Family Control ${tag} — Unified Parent & Child Windows MSI Installer`,
      defaultValue: `Vylos Family Control ${tag} — Unified Parent & Child Windows MSI Installer`,
    });
    if (p.isCancel(notesInput)) {
      p.cancel('Release cancelled.');
      process.exit(0);
    }
    releaseNotes = String(notesInput).trim();
  }
  if (!releaseNotes) {
    releaseNotes = `Vylos Family Control ${tag} — Unified Parent & Child Windows MSI Installer`;
  }

  p.note(
    [
      `Brand:        Vylos Family Control`,
      `Version:      v${currentVersion} -> ${tag}`,
      `Repository:   ${GITHUB_REPO_URL}`,
      `Deliverables: dist/Vylos-Setup.msi (Unified Parent/Child MSI)`,
      `              dist/Vylos-windows-amd64.exe & dist/Vylos-windows-arm64.exe`,
      `              dist/latest.json & dist/latest.yml (Auto-Updater Manifests)`,
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

  // Step 3: Build both Windows architectures (amd64 & arm64) + root Vylos.exe
  const distDir = path.join(rootDir, 'dist');
  fs.mkdirSync(distDir, { recursive: true });

  const amd64Exe = path.join(distDir, 'Vylos-windows-amd64.exe');
  const arm64Exe = path.join(distDir, 'Vylos-windows-arm64.exe');
  const rootExe = path.join(rootDir, 'Vylos.exe');
  const ldflags = `-s -w -X familycontrol/backend/updater.AppVersion=${nextVersion}`;

  s.start('Compiling Vylos binaries (windows/amd64 & windows/arm64)...');
  execFileSync('go', ['build', `-ldflags=${ldflags}`, '-o', amd64Exe, '.'], {
    cwd: rootDir,
    env: { ...process.env, GOOS: 'windows', GOARCH: 'amd64' },
    stdio: 'pipe',
  });
  execFileSync('go', ['build', `-ldflags=${ldflags}`, '-o', arm64Exe, '.'], {
    cwd: rootDir,
    env: { ...process.env, GOOS: 'windows', GOARCH: 'arm64' },
    stdio: 'pipe',
  });
  fs.copyFileSync(amd64Exe, rootExe);
  s.stop('Compiled Vylos-windows-amd64.exe, Vylos-windows-arm64.exe, and Vylos.exe.');

  // Step 4: Build Unified MSI Installer (dist/Vylos-Setup.msi)
  s.start('Building unified MSI installer (dist/Vylos-Setup.msi)...');
  const msiPath = path.join(distDir, 'Vylos-Setup.msi');
  buildMsi({
    version: nextVersion,
    sourceExePath: amd64Exe,
    outputMsiPath: msiPath,
  });
  s.stop('Created unified MSI installer: dist/Vylos-Setup.msi');

  // Step 5: Generate latest.json and latest.yml for the Auto-Updater
  s.start('Generating latest.json and latest.yml updater manifests...');
  const pubDate = new Date().toISOString();
  const msiSha256 = sha256File(msiPath);
  const msiSha512 = sha512Base64File(msiPath);
  const msiSize = fs.statSync(msiPath).size;
  const amd64Sha256 = sha256File(amd64Exe);
  const arm64Sha256 = sha256File(arm64Exe);

  const msiDownloadUrl = `${GITHUB_REPO_URL}/releases/download/${tag}/Vylos-Setup.msi`;
  const amd64DownloadUrl = `${GITHUB_REPO_URL}/releases/download/${tag}/Vylos-windows-amd64.exe`;
  const arm64DownloadUrl = `${GITHUB_REPO_URL}/releases/download/${tag}/Vylos-windows-arm64.exe`;

  const latestJson = {
    version: nextVersion,
    tag,
    notes: releaseNotes,
    pub_date: pubDate,
    url: msiDownloadUrl,
    sha256: msiSha256,
    platforms: {
      'windows-x86_64': {
        url: msiDownloadUrl,
        exe_url: amd64DownloadUrl,
        sha256: msiSha256,
        exe_sha256: amd64Sha256,
      },
      'windows-aarch64': {
        url: msiDownloadUrl,
        exe_url: arm64DownloadUrl,
        sha256: msiSha256,
        exe_sha256: arm64Sha256,
      },
    },
  };

  const latestJsonPath = path.join(distDir, 'latest.json');
  fs.writeFileSync(latestJsonPath, JSON.stringify(latestJson, null, 2) + '\n', 'utf8');

  const latestYml = [
    `version: ${nextVersion}`,
    `releaseDate: '${pubDate}'`,
    `path: Vylos-Setup.msi`,
    `url: ${msiDownloadUrl}`,
    `sha256: ${msiSha256}`,
    `sha512: ${msiSha512}`,
    `files:`,
    `  - url: Vylos-Setup.msi`,
    `    sha256: ${msiSha256}`,
    `    sha512: ${msiSha512}`,
    `    size: ${msiSize}`,
    `  - url: Vylos-windows-amd64.exe`,
    `    sha256: ${amd64Sha256}`,
    `  - url: Vylos-windows-arm64.exe`,
    `    sha256: ${arm64Sha256}`,
    `notes: ${JSON.stringify(releaseNotes)}`,
    '',
  ].join('\n');

  const latestYmlPath = path.join(distDir, 'latest.yml');
  fs.writeFileSync(latestYmlPath, latestYml, 'utf8');
  s.stop('Generated dist/latest.json and dist/latest.yml.');

  if (opts.dryRun) {
    p.outro(`Dry run complete! Built ${msiPath}, ${latestJsonPath}, and ${latestYmlPath}.`);
    return;
  }

  // Step 6: Git commit, tag & GitHub Release upload
  s.start(`Publishing ${tag} to GitHub (${GITHUB_REPO})...`);
  try {
    // Ensure git repo is initialized and remote is configured
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
    } catch {
      // ignore remote check error
    }

    execFileSync('git', ['add', 'package.json', 'frontend/package.json', 'wails.json', 'installer/Vylos.wxs'], {
      cwd: rootDir,
      stdio: 'pipe',
    });

    try {
      execFileSync('git', ['commit', '-m', `chore(release): ${tag}`], { cwd: rootDir, stdio: 'pipe' });
    } catch {
      // Nothing to commit if version was kept
    }

    try {
      execFileSync('git', ['tag', '-f', tag], { cwd: rootDir, stdio: 'pipe' });
    } catch {
      // ignore tag error
    }

    try {
      execFileSync('git', ['push', 'origin', 'HEAD', '--tags', '--force'], { cwd: rootDir, stdio: 'pipe' });
    } catch {
      // Continue to gh release create even if git push needs upstream branch
    }

    const gh = findGhCli();
    const releaseAssets = [msiPath, latestJsonPath, latestYmlPath, amd64Exe, arm64Exe];

    // Check if release already exists
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
    s.stop(`Local MSI & manifests built; GitHub upload encountered an error: ${err.message}`);
    process.exitCode = 1;
    return;
  }

  p.outro(`Successfully built and released Vylos Family Control ${tag}!`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
