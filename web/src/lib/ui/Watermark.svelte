<script lang="ts">
  // 全局水印：对齐公司 Vue 的 XcWatermark（element-plus el-watermark 的封装）：
  //   content = `${用户名}  ${当前时间}`（时间每秒刷新）/ gap 100×100 / 旋转 -22° / 16px 等宽字体
  //   颜色 light: rgba(0,0,0,.08)，dark: rgba(255,255,255,.08)
  // 做法与 el-watermark 一致：canvas 画一块「文字 + 间距」的瓦片，铺成 repeating background，
  // 而不是用 DOM 拼文字，这样倾斜文字不会被裁切、也不会拖慢滚动。
  import { config } from '../systemConfig.svelte';
  import { currentTheme, type Theme } from '../theme';

  interface Props {
    username?: string;
  }
  let { username = 'admin' }: Props = $props();

  const ROTATE = -22; // el-watermark 默认角度
  const GAP_X = 100; // 与 Vue 版 :gap="[100, 100]" 一致
  const GAP_Y = 100;
  const FONT_SIZE = 16;
  const FONT_FAMILY = 'Monaco, Consolas, monospace';

  function stamp(): string {
    const now = new Date();
    const pad = (n: number): string => String(n).padStart(2, '0');
    return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
  }

  // 时间每秒变一次，瓦片跟着重画（和 Vue 版 setInterval 1s 一致）
  let now = $state(stamp());
  $effect(() => {
    const timer = setInterval(() => (now = stamp()), 1000);
    return () => clearInterval(timer);
  });

  // 主题只在重画时读一次：主题切换后最迟 1 秒颜色跟上
  const tile = $derived(buildTile(`${username}  ${now}`, currentTheme()));

  function buildTile(text: string, theme: Theme): string {
    const color = theme === 'dark' ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.08)';
    const font = `${FONT_SIZE}px ${FONT_FAMILY}`;
    const dpr = Math.min(2, window.devicePixelRatio || 1);

    const measure = document.createElement('canvas').getContext('2d');
    if (!measure) return '';
    measure.font = font;
    const textWidth = Math.ceil(measure.measureText(text).width);

    // 瓦片 = 文字外框 + 间距；按元素中心旋转，保证倾斜文字完整落在瓦片内
    const markWidth = textWidth + 24;
    const markHeight = FONT_SIZE * 2;
    const tileWidth = markWidth + GAP_X;
    const tileHeight = markHeight + GAP_Y;

    const canvas = document.createElement('canvas');
    canvas.width = Math.ceil(tileWidth * dpr);
    canvas.height = Math.ceil(tileHeight * dpr);
    const ctx = canvas.getContext('2d');
    if (!ctx) return '';

    ctx.scale(dpr, dpr);
    ctx.translate(tileWidth / 2, tileHeight / 2);
    ctx.rotate((ROTATE * Math.PI) / 180);
    ctx.font = font;
    ctx.fillStyle = color;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, 0, 0);

    return canvas.toDataURL('image/png');
  }
</script>

{#if config().watermark && tile}
  <div
    class="pointer-events-none fixed inset-0 z-[100] select-none"
    style={`background-image: url(${tile}); background-repeat: repeat;`}
    aria-hidden="true"
  ></div>
{/if}
