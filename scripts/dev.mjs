// 单窗口开发入口：构建前端 -> 同步到根 dist -> 前台启动 go run ./cmd/server。
// 相比两个 cmd 窗口的方案：只占一个终端、日志按顺序输出、Ctrl+C 一次全退出。
// 用法：pnpm dev（首次或前端有改动时用）；只想跳过构建直接起服务用 pnpm dev:server。
// 附加参数 --build-only：只构建并同步 dist，不启动后端（用于刷新静态资源）。
import { execSync, spawn } from 'node:child_process';
import { cpSync, rmSync } from 'node:fs';

const buildOnly = process.argv.includes('--build-only');

console.log('[1/2] building frontend (pnpm -C web build) ...');
execSync('pnpm -C web build', { stdio: 'inherit' });

console.log('[2/2] syncing web/dist -> dist ...');
// 先删再复制：目标已存在时 cpSync 会嵌套成 dist/dist，必须清掉旧目录。
rmSync('dist', { recursive: true, force: true });
cpSync('web/dist', 'dist', { recursive: true });

if (buildOnly) {
  console.log('\ndone (--build-only), dist is up to date.');
  process.exit(0);
}

console.log('\nstarting go run ./cmd/server ... (Ctrl+C to stop)\n');
const child = spawn('go', ['run', './cmd/server'], { stdio: 'inherit', shell: true });

child.on('exit', (code) => process.exit(code ?? 0));
for (const sig of ['SIGINT', 'SIGTERM']) {
  process.on(sig, () => child.kill(sig));
}
