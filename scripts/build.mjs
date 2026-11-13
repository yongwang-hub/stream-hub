// One-command build: frontend -> copy dist -> backend -> config.
// Produces one directory per target: package/stream-hub-<tag>-<version>.<YYMMDD>/
//   containing stream-hub(.exe), dist/, config/config.yaml — ready to zip and hand off.
// Flags: --win (Windows/amd64), --amd64 (Linux/amd64), --arm64 (Linux/arm64).
//   Version: default = root package.json "version", auto-prefixed with "V"; --version=X.Y.Z overrides (also gets the V).
//   --skip-build skips the frontend pnpm build (used by build:all; dist copy still runs).
import { execSync } from 'node:child_process';
import { cpSync, mkdirSync, readFileSync, rmSync } from 'node:fs';

const args = process.argv.slice(2);
const hasWin = args.includes('--win');
const hasAmd = args.includes('--amd64');
const hasArm = args.includes('--arm64');
const skipBuild = args.includes('--skip-build');

// 版本来源：默认读根 package.json 的 version（改这里即可）；--version=X.Y.Z 可覆盖。
// 目录名统一加 V 前缀（如 package.json 是 0.1.0 → V0.1.0）；已带 V 则不重复加。
function pkgVersion() {
  try {
    return JSON.parse(readFileSync('package.json', 'utf8')).version || '0.0.0';
  } catch {
    return '0.0.0';
  }
}
const versionArg = args.find((a) => a.startsWith('--version='));
const rawVersion = versionArg ? versionArg.slice('--version='.length) : pkgVersion();
const version = rawVersion.startsWith('V') ? rawVersion : 'V' + rawVersion;

const d = new Date();
const yymmdd = `${String(d.getFullYear()).slice(2)}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;

const GOOS = (hasAmd || hasArm) ? 'linux' : 'windows';
const GOARCH = hasArm ? 'arm64' : 'amd64';
const tag = `${GOOS}-${GOARCH}`;
const ext = GOOS === 'windows' ? '.exe' : '';
const dirName = `stream-hub-${tag}-${version}.${yymmdd}`;
const dir = `package/${dirName}`;
const binName = `stream-hub${ext}`;
const out = `${dir}/${binName}`;

mkdirSync(dir, { recursive: true });
mkdirSync(`${dir}/config`, { recursive: true });

if (!skipBuild) {
  console.log('[1/4] building frontend (pnpm -C web build) ...');
  execSync('pnpm -C web build', { stdio: 'inherit' });
}

console.log(`[2/4] copying dist -> ${dir}/dist ...`);
rmSync(`${dir}/dist`, { recursive: true, force: true });
mkdirSync(`${dir}/dist`, { recursive: true });
cpSync('web/dist', `${dir}/dist`, { recursive: true });

console.log(`[3/4] building backend ${tag} (CGO_ENABLED=0) -> ${out} ...`);
execSync(`go build -trimpath -ldflags "-s -w -X main.Version=${version}" -o ${out} ./cmd/server`, {
  stdio: 'inherit',
  env: { ...process.env, GOOS, GOARCH, CGO_ENABLED: '0' },
});

console.log('[4/4] copying config ...');
cpSync('config/config.yaml', `${dir}/config/config.yaml`);

console.log(`\ndone. package at ${dir}/  ->  cd ${dir} && ./${binName}`);
