import * as p from '@clack/prompts';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const webDir = path.join(rootDir, 'website');
const GITHUB_REPO = 'Tenkayro/Vylos';
const GITHUB_REPO_URL = `https://github.com/${GITHUB_REPO}`;
const CUSTOM_DOMAIN = 'vylos.tenkayro.de';

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

function syncWebsiteVersion() {
  const pkg = JSON.parse(fs.readFileSync(path.join(rootDir, 'package.json'), 'utf8'));
  const ver = (pkg.version || '1.0.0').replace(/^v/, '');

  const appJsPath = path.join(webDir, 'app.js');
  if (fs.existsSync(appJsPath)) {
    let js = fs.readFileSync(appJsPath, 'utf8');
    js = js.replace(/let version = '[^']+';/, `let version = '${ver}';`);
    fs.writeFileSync(appJsPath, js, 'utf8');
  }

  const indexHtmlPath = path.join(webDir, 'index.html');
  if (fs.existsSync(indexHtmlPath)) {
    let html = fs.readFileSync(indexHtmlPath, 'utf8');
    html = html.replace(/v\d+\.\d+\.\d+ · Windows 10\/11/g, `v${ver} · Windows 10/11`);
    fs.writeFileSync(indexHtmlPath, html, 'utf8');
  }

  fs.writeFileSync(path.join(webDir, 'CNAME'), `${CUSTOM_DOMAIN}\n`, 'utf8');
  return ver;
}

function startLocalPreviewServer(port = 41888) {
  const mimeTypes = {
    '.html': 'text/html; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.js': 'application/javascript; charset=utf-8',
    '.svg': 'image/svg+xml',
    '.json': 'application/json; charset=utf-8',
  };

  const server = http.createServer((req, res) => {
    const reqPath = (req.url || '/').split('?')[0];
    const relPath = reqPath === '/' ? 'index.html' : reqPath.replace(/^\/+/, '');
    const filePath = path.join(webDir, relPath);

    if (!filePath.startsWith(webDir) || !fs.existsSync(filePath) || fs.statSync(filePath).isDirectory()) {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('Not found');
      return;
    }

    const ext = path.extname(filePath).toLowerCase();
    res.writeHead(200, { 'Content-Type': mimeTypes[ext] || 'application/octet-stream' });
    res.end(fs.readFileSync(filePath));
  });

  server.listen(port, '127.0.0.1', () => {
    p.note(`Local preview running at: http://127.0.0.1:${port}\nPress Ctrl+C to stop.`, 'Vylos Website Preview');
  });
}

async function main() {
  const args = process.argv.slice(2);
  const isDryRun = args.includes('--dry-run');
  const isPreview = args.includes('--preview');
  const autoYes = args.includes('--yes') || args.includes('-y') || args.includes('--deploy');

  p.intro('Vylos Family Control — GitHub Pages Website Manager (vylos.tenkayro.de)');

  const version = syncWebsiteVersion();

  if (isPreview) {
    startLocalPreviewServer();
    return;
  }

  if (isDryRun) {
    p.outro(`Synced website/ assets (v${version}) and CNAME (${CUSTOM_DOMAIN}).`);
    return;
  }

  let mode = 'deploy';
  if (!autoYes) {
    const selected = await p.select({
      message: `Manage Vylos Website (${CUSTOM_DOMAIN} · Release v${version}):`,
      options: [
        {
          value: 'deploy',
          label: `Deploy to GitHub Pages (https://${CUSTOM_DOMAIN})`,
          hint: 'Pushes website/ & triggers GitHub Actions workflow_dispatch manually',
        },
        {
          value: 'preview',
          label: 'Start Local Preview Server (http://127.0.0.1:41888)',
          hint: 'Test the landing page locally in your browser',
        },
        {
          value: 'sync',
          label: 'Sync Version & CNAME Only',
          hint: 'Updates website/ files without deploying',
        },
      ],
    });

    if (p.isCancel(selected)) {
      p.cancel('Cancelled.');
      process.exit(0);
    }
    mode = String(selected);
  }

  if (mode === 'preview') {
    startLocalPreviewServer();
    return;
  }

  if (mode === 'sync') {
    p.outro(`Website files synced to v${version} in ./website`);
    return;
  }

  const s = p.spinner();
  s.start(`Pushing website files & triggering GitHub Actions Pages workflow for ${CUSTOM_DOMAIN}...`);

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

    execFileSync('git', ['add', 'website', '.github/workflows/pages.yml'], { cwd: rootDir, stdio: 'pipe' });
    try {
      execFileSync('git', ['commit', '-m', `chore(web): update Vylos landing page (v${version})`], {
        cwd: rootDir,
        stdio: 'pipe',
      });
    } catch {
      // No changes to commit
    }

    execFileSync('git', ['push', '-u', 'origin', 'HEAD'], { cwd: rootDir, stdio: 'pipe' });

    const gh = findGhCli();
    execFileSync(
      gh,
      ['workflow', 'run', 'pages.yml', '--repo', GITHUB_REPO, '-f', `reason=npm run web (v${version})`],
      { cwd: rootDir, stdio: 'inherit' }
    );

    s.stop(`Triggered GitHub Pages deployment for https://${CUSTOM_DOMAIN}`);
    p.outro(`Website deployment started! Live shortly at https://${CUSTOM_DOMAIN}`);
  } catch (err) {
    s.stop(`Deployment command finished with notice: ${err.message}`);
    process.exitCode = 1;
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
