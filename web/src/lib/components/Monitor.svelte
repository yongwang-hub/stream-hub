<script lang="ts">
  // 系统监控：只展示真实指标（2 秒刷新），无数据时空着，不摆静态示意。
  // 首屏 GET /api/v1/metrics 补历史，SSE system-metrics 频道收增量。
  import { onMount } from 'svelte';
  import Card from '../ui/Card.svelte';
  import Badge from '../ui/Badge.svelte';
  import Icon from '../ui/Icon.svelte';
  import {
    fetchMetricsSnapshot,
    subscribeMetrics,
    type MetricPoint,
    type MetricsStatus,
  } from '../metrics';

  const MAX_POINTS = 120; // 4 分钟 @2s，与后端环形缓冲一致

  let points = $state<MetricPoint[]>([]);
  let status = $state<MetricsStatus>('connecting');
  let latest = $derived<MetricPoint | null>(points.length ? points[points.length - 1] : null);

  let cpuPath = $derived(linePath(points.map((p) => p.cpu), 100));
  let memPath = $derived(linePath(points.map((p) => p.mem_percent), 100));
  let netMax = $derived(Math.max(1, ...points.map((p) => p.net_rx), ...points.map((p) => p.net_tx)));
  let rxPath = $derived(linePath(points.map((p) => p.net_rx), netMax));
  let txPath = $derived(linePath(points.map((p) => p.net_tx), netMax));
  let startAt = $derived(points.length ? points[0].time.slice(11, 19) : '');
  let endAt = $derived(latest ? latest.time.slice(11, 19) : '');

  function linePath(values: number[], max: number): string {
    if (values.length < 2) return '';
    return values
      .map((v, i) => {
        const x = (i / (values.length - 1)) * 300;
        const y = 110 - Math.min(1, Math.max(0, v / max)) * 110;
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
  }

  function gib(bytes: number): string {
    return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  }

  function pct(v: number): string {
    return `${v.toFixed(1)}%`;
  }

  onMount(() => {
    void fetchMetricsSnapshot()
      .then((snapshot) => {
        points = snapshot.history.slice(-MAX_POINTS);
      })
      .catch(() => {
        status = 'closed';
      });

    const stream = subscribeMetrics(
      (point) => {
        points = [...points, point].slice(-MAX_POINTS);
      },
      (next) => {
        status = next;
      },
    );
    return () => stream.close();
  });
</script>

<div class="space-y-5">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div>
      <h2 class="text-xl font-semibold tracking-tight">系统监控</h2>
      <p class="mt-1 text-sm text-muted-foreground">真实系统指标 · 每 2 秒刷新</p>
    </div>
    {#if status === 'open'}
      <Badge variant="success">● 实时采集中</Badge>
    {:else if status === 'connecting'}
      <Badge variant="secondary">○ 连接中…</Badge>
    {:else}
      <Badge variant="warning">○ 指标采集未连接</Badge>
    {/if}
  </div>

  {#if !latest}
    <div class="rounded-xl border border-border bg-card px-6 py-16 text-center">
      <Icon name="monitor" size={32} class="mx-auto text-muted-foreground" />
      <p class="mt-4 text-base font-medium">
        {status === 'connecting' ? '正在连接指标服务…' : '指标采集未连接'}
      </p>
      <p class="mt-1 text-sm text-muted-foreground">暂无可用指标数据</p>
    </div>
  {:else}
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <Card title="系统概览" description="CPU / 内存实时占用">
        <div class="flex items-center justify-around gap-4 pt-1">
          <div class="text-center">
            <div class="mx-auto grid h-24 w-24 place-items-center rounded-full border-[9px] border-primary text-base font-bold">
              {pct(latest.cpu)}
            </div>
            <div class="mt-3 text-xs text-muted-foreground">CPU 使用率</div>
          </div>
          <div class="text-center">
            <div class="mx-auto grid h-24 w-24 place-items-center rounded-full border-[9px] border-primary/60 text-base font-bold">
              {pct(latest.mem_percent)}
            </div>
            <div class="mt-3 text-xs text-muted-foreground">内存使用率</div>
          </div>
        </div>
      </Card>

      <Card
        title="系统负载"
        description={latest.load1 < 0 ? '当前平台不支持负载指标' : '1 / 5 / 15 分钟平均负载'}
      >
        <div class="flex h-36 items-end gap-3 pt-1">
          {#if latest.load1 >= 0}
            {#each [{ label: '1 分钟', value: latest.load1 }, { label: '5 分钟', value: latest.load5 }, { label: '15 分钟', value: latest.load15 }] as item}
              <div class="flex h-full flex-1 flex-col items-center justify-end gap-2">
                <span class="text-xs text-muted-foreground">{item.value.toFixed(2)}</span>
                <div class="w-full rounded-t-md bg-primary/70" style="height: {Math.min(100, item.value * 20)}%"></div>
                <span class="text-xs whitespace-nowrap text-muted-foreground">{item.label}</span>
              </div>
            {/each}
          {:else}
            <div class="m-auto text-sm text-muted-foreground">当前平台不支持负载指标</div>
          {/if}
        </div>
      </Card>

      <Card title="CPU 使用" description={`当前 ${pct(latest.cpu)}`}>
        <svg class="h-32 w-full text-primary" viewBox="0 0 300 110" preserveAspectRatio="none">
          <polygon points="0,110 {cpuPath} 300,110" fill="currentColor" opacity="0.12" />
          <polyline points={cpuPath} fill="none" stroke="currentColor" stroke-width="2" />
        </svg>
        <div class="mt-2 flex justify-between text-xs text-muted-foreground">
          <span>{startAt}</span><span>{endAt}</span>
        </div>
      </Card>

      <Card
        title="存储使用"
        description={`系统盘 ${pct(latest.disk_percent)} · ${gib(latest.disk_used)} / ${gib(latest.disk_total)}`}
      >
        <div class="space-y-3.5 pt-1">
          <div class="grid grid-cols-[72px_1fr_52px] items-center gap-3 text-xs">
            <span class="text-muted-foreground">系统盘</span>
            <div class="h-2 overflow-hidden rounded-full bg-muted">
              <div class="h-full rounded-full bg-primary" style="width: {latest.disk_percent}%"></div>
            </div>
            <span class="text-right text-muted-foreground">{pct(latest.disk_percent)}</span>
          </div>
          <div class="grid grid-cols-[72px_1fr_52px] items-center gap-3 text-xs">
            <span class="text-muted-foreground">Go 堆</span>
            <div class="h-2 overflow-hidden rounded-full bg-muted">
              <div
                class="h-full rounded-full bg-primary/70"
                style="width: {latest.mem_total ? (latest.heap_alloc / latest.mem_total) * 100 : 0}%"
              ></div>
            </div>
            <span class="text-right text-muted-foreground">{gib(latest.heap_alloc)}</span>
          </div>
          <div class="text-xs text-muted-foreground">协程数 {latest.goroutines}</div>
        </div>
      </Card>

      <Card
        title="带宽使用 (Mbps)"
        description={`↓ ${latest.net_rx.toFixed(2)} / ↑ ${latest.net_tx.toFixed(2)}`}
      >
        <svg class="h-32 w-full text-primary" viewBox="0 0 300 110" preserveAspectRatio="none">
          <polygon points="0,110 {rxPath} 300,110" fill="currentColor" opacity="0.12" />
          <polyline points={rxPath} fill="none" stroke="currentColor" stroke-width="2" />
          <polyline points={txPath} fill="none" stroke="#f59e0b" stroke-width="1.5" stroke-dasharray="4 3" />
        </svg>
        <div class="mt-2 flex justify-between text-xs text-muted-foreground">
          <span>{startAt}</span><span>{endAt}</span>
        </div>
      </Card>

      <Card
        title="内存使用"
        description={`${gib(latest.mem_used)} / ${gib(latest.mem_total)}`}
      >
        <svg class="h-32 w-full text-primary" viewBox="0 0 300 110" preserveAspectRatio="none">
          <polygon points="0,110 {memPath} 300,110" fill="currentColor" opacity="0.12" />
          <polyline points={memPath} fill="none" stroke="currentColor" stroke-width="2" />
        </svg>
        <div class="mt-2 flex justify-between text-xs text-muted-foreground">
          <span>{startAt}</span><span>{endAt}</span>
        </div>
      </Card>
    </div>
  {/if}
</div>
