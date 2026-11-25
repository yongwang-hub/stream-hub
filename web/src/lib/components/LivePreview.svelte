<script lang="ts">
  // 实时预览：左侧本地设备列表 + 右侧分屏播放区。
  // 布局照公司 Vue 的 apps/zhzxpt/src/views/live-preview：
  //   左：搜索 + 设备列表 + 底部「显示方式」；右：分屏网格 + 底部工具栏。
  // 播放链路尚未接入：浏览器不能直连 RTSP，这里先只做「摆放设备 / 分屏 / 全屏」，
  // 窗口里显示占位信息，流媒体网关接上后再填 <video>。
  import { onMount } from 'svelte';
  import { api, type PreviewChannel } from '../api';
  import Button from '../ui/Button.svelte';
  import Icon from '../ui/Icon.svelte';
  import { inputClass } from '../ui/classes';
  import { toast } from '../ui/toast.svelte';
  import { cn } from '../utils';

  // 分屏模式：不做 8 分屏（用户确认）。
  // 6 分屏照 Vue 版排版：3×3 网格，第一个窗口占 2×2，其余 5 个各占一格。
  interface SplitMode {
    cells: number;
    cols: number;
    rows: number;
    featured: boolean;
  }
  const SPLITS: Record<number, SplitMode> = {
    1: { cells: 1, cols: 1, rows: 1, featured: false },
    4: { cells: 4, cols: 2, rows: 2, featured: false },
    6: { cells: 6, cols: 3, rows: 3, featured: true },
    9: { cells: 9, cols: 3, rows: 3, featured: false },
    16: { cells: 16, cols: 4, rows: 4, featured: false },
  };
  const SPLIT_KEYS = [1, 4, 6, 9, 16];

  // 窗口底色：照公司 Vue 版的浅蓝渐变（中间白、四周淡蓝），不放水印/产品 Logo。
  // 用固定浅色而不是主题色：视频墙的常规观感，浅底也能让“无画面”提示更克制。
  const CELL_BG =
    'radial-gradient(120% 120% at 50% 38%, #ffffff 0%, #f1f7fe 55%, #dfeaf9 100%)';

  let cameras = $state<PreviewChannel[]>([]);
  let loading = $state(false);
  let keyword = $state('');
  let displayMode = $state<'name' | 'rtsp'>('name');

  let split = $state(4);
  const mode = $derived(SPLITS[split]);
  let slots = $state<(PreviewChannel | null)[]>([null, null, null, null]);
  let selected = $state(0);

  const filtered = $derived.by(() => {
    const kw = keyword.trim().toLowerCase();
    if (!kw) return cameras;
    return cameras.filter(
      (c) => c.name.toLowerCase().includes(kw) || c.rtsp_url.toLowerCase().includes(kw),
    );
  });
  // 已加到画面里的设备 id：列表里高亮，避免同一台设备重复摆放。
  const onScreen = $derived(
    new Set(slots.filter((c): c is PreviewChannel => c !== null).map((c) => c.id)),
  );
  const usedCount = $derived(slots.filter((c) => c !== null).length);

  // 窗口左上角的时间 OSD：每秒刷新（照 Vue 版由上层计时后传下来）
  function stamp(): string {
    const d = new Date();
    const p = (n: number): string => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
  }
  let now = $state(stamp());
  $effect(() => {
    const timer = setInterval(() => (now = stamp()), 1000);
    return () => clearInterval(timer);
  });

  const describe = (err: unknown): string => (err instanceof Error ? err.message : String(err));

  async function load(): Promise<void> {
    loading = true;
    try {
      const result = await api.previewCameras();
      cameras = result.items ?? [];
    } catch (err) {
      toast.error(describe(err));
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });

  // 切换分屏：保留前 n 个已摆放的窗口，多出来的截断
  function setSplit(next: number): void {
    if (next === split) return;
    const cells = SPLITS[next].cells;
    const kept = slots.slice(0, cells);
    while (kept.length < cells) kept.push(null);
    slots = kept;
    selected = Math.min(selected, cells - 1);
    split = next;
  }

  // 双击设备 → 放进当前选中的窗口（替换该窗口原有画面）
  function assign(channel: PreviewChannel): void {
    if (!channel.enabled) {
      toast.error('设备已停用，请先在「设备管理 → 摄像机管理」中启用');
      return;
    }
    const index = selected;
    slots = slots.map((cur, i) => (i === index ? channel : cur));
  }

  function clearSlot(index: number): void {
    slots = slots.map((cur, i) => (i === index ? null : cur));
  }

  function clearAll(): void {
    slots = slots.map(() => null);
  }

  // 全屏只针对右侧播放区
  let playerEl = $state<HTMLElement | null>(null);
  let fullscreen = $state(false);
  function toggleFullscreen(): void {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void playerEl?.requestFullscreen?.();
  }
  $effect(() => {
    const onChange = (): void => {
      fullscreen = document.fullscreenElement !== null;
    };
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  });
</script>

{#snippet glyph(m: SplitMode)}
  <!-- 分屏图标：按行列画小格，6 分屏第一个格占 2×2，和真实排版一致 -->
  <span
    class="grid h-3.5 w-3.5 shrink-0 gap-[1.5px] rounded-[2px] border border-current p-[1.5px]"
    style={`grid-template-columns: repeat(${m.cols}, minmax(0, 1fr)); grid-template-rows: repeat(${m.rows}, minmax(0, 1fr));`}
  >
    {#each Array.from({ length: m.cells }) as _, i}
      <span
        class={cn('rounded-[1px] bg-current opacity-50', m.featured && i === 0 && 'col-span-2 row-span-2')}
      ></span>
    {/each}
  </span>
{/snippet}

<div class="flex min-h-full flex-1 flex-col">
  <div class="flex min-h-0 flex-1 gap-6">
    <!-- 左侧：本地设备列表 -->
    <aside
      class="flex w-[280px] shrink-0 flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm"
    >
      <div class="shrink-0 border-b border-border p-3">
        <div class="relative">
          <input class={cn(inputClass, 'pr-9')} placeholder="请输入设备名称" bind:value={keyword} />
          <Icon
            name="search"
            size={15}
            class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
        </div>
      </div>

      <div class="ui-scroll min-h-0 flex-1 overflow-y-auto p-2">
        {#if loading}
          <p class="px-2 py-6 text-center text-xs text-muted-foreground">加载中…</p>
        {:else if filtered.length === 0}
          <p class="px-2 py-6 text-center text-xs text-muted-foreground">
            {keyword ? '没有匹配的设备' : '暂无设备，请先在「设备管理 → 摄像机管理」中添加'}
          </p>
        {:else}
          {#each filtered as cam (cam.id)}
            <button
              type="button"
              class={cn(
                'flex h-9 w-full items-center gap-2 rounded-lg px-2.5 text-left text-sm transition-colors',
                cam.enabled ? 'text-foreground hover:bg-muted' : 'text-muted-foreground/70',
                onScreen.has(cam.id) && 'bg-primary/10 font-medium text-primary hover:bg-primary/15',
              )}
              title={cam.rtsp_url}
              ondblclick={() => assign(cam)}
            >
              <span
                class={cn(
                  'h-1.5 w-1.5 shrink-0 rounded-full',
                  cam.enabled ? 'bg-emerald-500' : 'bg-muted-foreground/40',
                )}
              ></span>
              <span class="truncate">{displayMode === 'name' ? cam.name : cam.rtsp_url}</span>
            </button>
          {/each}
        {/if}
      </div>

      <!-- 显示方式：照 Vue 版列表底部的「摄像机名称 / OSD名称」，本项目用名称 / 流地址 -->
      <div class="shrink-0 border-t border-border px-3 py-2.5">
        <div class="mb-1.5 text-xs text-muted-foreground">显示方式</div>
        <div class="flex items-center gap-4 text-sm">
          <label class="flex cursor-pointer items-center gap-1.5">
            <input type="radio" class="h-4 w-4 accent-primary" bind:group={displayMode} value="name" />
            摄像机名称
          </label>
          <label class="flex cursor-pointer items-center gap-1.5">
            <input type="radio" class="h-4 w-4 accent-primary" bind:group={displayMode} value="rtsp" />
            RTSP 地址
          </label>
        </div>
      </div>
    </aside>

    <!-- 右侧：分屏播放区（浅蓝渐变底，照 Vue 版视频墙观感） -->
    <section
      bind:this={playerEl}
      class="flex min-w-0 flex-1 flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm"
    >
      <div
        class="grid min-h-0 flex-1 gap-1.5 bg-[#e8f0fb] p-1.5"
        style={`grid-template-columns: repeat(${mode.cols}, minmax(0, 1fr)); grid-template-rows: repeat(${mode.rows}, minmax(0, 1fr));`}
      >
        {#each slots as channel, index (index)}
          <div
            class={cn(
              'relative overflow-hidden rounded ring-1 ring-black/5 transition-shadow',
              // 6 分屏照 Vue 版：3×3 网格，第一个窗口占左上 2×2，右侧竖排 2 个、底部横排 3 个
              mode.featured && index === 0 && 'col-span-2 row-span-2',
              index === selected && 'ring-2 ring-primary',
            )}
            style={`background-image: ${CELL_BG};`}
            role="presentation"
            onclick={() => (selected = index)}
          >
            {#if channel}
              <span
                class="pointer-events-none absolute left-2 top-1.5 z-10 font-mono text-[11px] text-slate-500"
                >{now}</span
              >
              <button
                type="button"
                class="absolute right-2 top-1.5 z-10 rounded px-1.5 text-[11px] text-slate-400 transition-colors hover:bg-black/5 hover:text-slate-600"
                onclick={(event) => {
                  event.stopPropagation();
                  clearSlot(index);
                }}
              >
                关闭
              </button>
              <div class="flex h-full flex-col items-center justify-center gap-1.5 px-4 text-center">
                <Icon name="camera" size={26} class="text-slate-300" />
                <span class="max-w-full truncate text-sm text-slate-600">{channel.name}</span>
                <span class="line-clamp-2 text-[11px] text-slate-400">画面暂未接入</span>
              </div>
              <div
                class="absolute inset-x-0 bottom-0 z-10 truncate bg-white/60 px-2 py-1 text-[11px] text-slate-600"
              >
                {displayMode === 'name' ? channel.name : channel.rtsp_url}
              </div>
            {:else}
              <div
                class="flex h-full items-center justify-center px-3 text-center text-xs text-slate-400/80"
              >
                双击左侧设备添加到当前窗口
              </div>
            {/if}
          </div>
        {/each}
      </div>

      <!-- 底部工具栏：左 分屏模式 / 中 已用窗口 / 右 清空 + 全屏 -->
      <div class="flex h-12 shrink-0 items-center justify-between gap-3 border-t border-border px-3">
        <div class="flex items-center gap-1">
          {#each SPLIT_KEYS as key (key)}
            <button
              type="button"
              class={cn(
                'flex h-9 items-center gap-1.5 rounded-lg px-2 text-[11px] transition-colors',
                split === key ? 'text-primary' : 'text-muted-foreground hover:bg-muted',
              )}
              onclick={() => setSplit(key)}
            >
              {@render glyph(SPLITS[key])}
              <span>{key}分屏</span>
            </button>
          {/each}
        </div>
        <span class="text-xs text-muted-foreground">已用 {usedCount} / {mode.cells} 个窗口</span>
        <div class="flex items-center gap-1">
          <Button variant="ghost" size="sm" disabled={usedCount === 0} onclick={clearAll}>清空</Button>
          <Button variant="ghost" size="sm" icon="fullscreen" onclick={toggleFullscreen}>
            {fullscreen ? '退出全屏' : '全屏'}
          </Button>
        </div>
      </div>
    </section>
  </div>
</div>
