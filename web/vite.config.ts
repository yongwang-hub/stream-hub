import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

// 开发态把后端前缀代理到 Go 服务，浏览器只看到同源，省掉预检与跨域配置。
// 后端 CORS 白名单已包含 http://localhost:5173，直连也能工作。
const backend = process.env.STREAM_HUB_BACKEND ?? 'http://127.0.0.1:8080';
const proxy = Object.fromEntries(
  ['/api', '/login', '/me', '/ping', '/healthz', '/readyz'].map((prefix) => [
    prefix,
    { target: backend, changeOrigin: true },
  ]),
);

export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  server: { port: 5173, strictPort: true, proxy },
  // 生产包不带 sourcemap：产物要嵌进 exe 分发，不暴露源码。
  build: { target: 'es2022', sourcemap: false },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
