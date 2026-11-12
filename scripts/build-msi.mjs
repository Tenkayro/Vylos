import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

function findWixTool(toolName) {
  const candidates = [
    path.join(os.homedir(), 'AppData', 'Local', 'tauri', 'WixTools314', toolName),
    path.join('C:\\Program Files (x86)\\WiX Toolset v3.14\\bin', toolName),
    path.join('C:\\Program Files (x86)\\WiX Toolset v3.11\\bin', toolName),
  ];
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  return toolName;
}

export function buildMsi({ version, edition = 'parent', sourceExePath, outputMsiPath } = {}) {
  const pkg = JSON.parse(fs.readFileSync(path.join(rootDir, 'package.json'), 'utf8'));
  const ver = (version || pkg.version || '1.0.0').replace(/^v/, '');
  const distDir = path.join(rootDir, 'dist');
  fs.mkdirSync(distDir, { recursive: true });

  const isChild = edition === 'child';
  const defaultExeName = isChild ? 'Vylos-Child.exe' : 'Vylos-Parent.exe';
  const productTitle = isChild ? 'Vylos Family Control (Child)' : 'Vylos Family Control (Parent)';
  const defaultMsiName = isChild ? 'Vylos-Child.msi' : 'Vylos-Parent.msi';

  const srcExe = sourceExePath || path.join(distDir, defaultExeName);
  const outMsi = outputMsiPath || path.join(distDir, defaultMsiName);
  const wxsPath = path.join(rootDir, 'installer', 'Vylos.wxs');
  const wixObjPath = path.join(distDir, `Vylos-${edition}.wixobj`);

  if (!fs.existsSync(srcExe)) {
    throw new Error(`Executable not found at ${srcExe}. Build the binary first.`);
  }

  const candle = findWixTool('candle.exe');
  const light = findWixTool('light.exe');

  console.log(`Building MSI installer (${path.basename(outMsi)}) [${edition.toUpperCase()}] v${ver}...`);
  execFileSync(
    candle,
    [
      '-nologo',
      `-dProductVersion=${ver}`,
      `-dEdition=${edition}`,
      `-dProductTitle=${productTitle}`,
      `-dExeName=${defaultExeName}`,
      `-dSourceExe=${srcExe}`,
      '-out',
      wixObjPath,
      wxsPath,
    ],
    { cwd: rootDir, stdio: 'inherit' }
  );

  execFileSync(
    light,
    [
      '-nologo',
      '-sval',
      '-out',
      outMsi,
      wixObjPath,
    ],
    { cwd: rootDir, stdio: 'inherit' }
  );

  try {
    fs.rmSync(wixObjPath, { force: true });
    fs.rmSync(outMsi.replace(/\.msi$/i, '.wixpdb'), { force: true });
  } catch {
    // ignore cleanup errors
  }

  console.log(`MSI installer created: ${outMsi}`);
  return outMsi;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  buildMsi({ edition: 'parent' });
  buildMsi({ edition: 'child' });
}
