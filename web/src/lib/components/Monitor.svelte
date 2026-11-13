<script lang="ts">
  // 系统监控：本轮仍为静态示意（不接指标接口），但换成统一卡片/主题色/精细化图表。
  import Card from '../ui/Card.svelte';
  import Badge from '../ui/Badge.svelte';

  const loads = [
    { label: '直播', value: 42 },
    { label: '回放', value: 18 },
    { label: '播放', value: 68 },
    { label: '录像', value: 35 },
    { label: 'H265', value: 54 },
    { label: '级联', value: 26 },
  ];
  const disks = [
    { name: '/home', used: 84 },
    { name: '/var', used: 72 },
    { name: '/opt', used: 66 },
    { name: '/root', used: 58 },
    { name: '/data', used: 40 },
    { name: '/', used: 22 },
  ];
  const cpuLine = '0,88 25,86 50,58 75,84 100,86 125,83 150,85 175,84 200,86 225,83 250,85 275,84 300,86';
  const memLine = '0,72 25,71 50,72 75,70 100,71 125,72 150,70 175,71 200,72 225,70 250,71 275,72 300,71';
  const bwLine = '0,96 25,20 50,68 75,66 100,67 125,65 150,68 175,66 200,67 225,66 250,68 275,96 300,96';
</script>

<div class="space-y-5">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div>
      <h2 class="text-xl font-semibold tracking-tight">系统监控</h2>
      <p class="mt-1 text-sm text-muted-foreground">服务运行概览 · 当前为静态示意数据</p>
    </div>
    <Badge variant="success">● 服务运行中</Badge>
  </div>

  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    <Card title="通道统计" description="在线率与在线通道数">
      <div class="flex items-center justify-around gap-4 pt-1">
        {#each [{ value: '100%', label: '在线率' }, { value: '58', label: '在线通道' }] as ring}
          <div class="text-center">
            <div class="mx-auto grid h-24 w-24 place-items-center rounded-full border-[9px] border-primary text-base font-bold">
              {ring.value}
            </div>
            <div class="mt-3 text-xs text-muted-foreground">{ring.label}</div>
          </div>
        {/each}
      </div>
    </Card>

    <Card title="负载统计" description="各业务模块当前负载（%）">
      <div class="flex h-36 items-end gap-3 pt-1">
        {#each loads as item}
          <div class="flex h-full flex-1 flex-col items-center justify-end gap-2">
            <span class="text-xs text-muted-foreground">{item.value}</span>
            <div
              class="w-full rounded-t-md bg-primary/70 transition-colors hover:bg-primary"
              style="height: {item.value * 0.85}%"
            ></div>
            <span class="text-xs whitespace-nowrap text-muted-foreground">{item.label}</span>
          </div>
        {/each}
      </div>
    </Card>

    <Card title="CPU 使用" description="近 1 分钟趋势">
      <svg class="h-32 w-full text-primary" viewBox="0 0 300 110" preserveAspectRatio="none">
        <polygon points="0,110 {cpuLine} 300,110" fill="currentColor" opacity="0.12" />
        <polyline points={cpuLine} fill="none" stroke="currentColor" stroke-width="2" />
      </svg>
      <div class="mt-2 flex justify-between text-xs text-muted-foreground">
        <span>15:14:14</span><span>15:14:38</span><span>15:15:02</span>
      </div>
    </Card>

    <Card title="存储使用" description="各挂载点磁盘占用">
      <div class="space-y-3.5 pt-1">
        {#each disks as disk}
          <div class="grid grid-cols-[56px_1fr_40px] items-center gap-3 text-xs">
            <span class="text-muted-foreground">{disk.name}</span>
            <div class="h-2 overflow-hidden rounded-full bg-muted">
              <div class="h-full rounded-full bg-primary" style="width: {disk.used}%"></div>
            </div>
            <span class="text-right text-muted-foreground">{disk.used}%</span>
          </div>
        {/each}
      </div>
    </Card>

    <Card title="带宽使用 (Mbps)" description="上下行流量趋势">
      <svg class="h-32 w-full text-primary" viewBox="0 0 300 110" preserveAspectRatio="none">
        <polygon points="0,110 {bwLine} 300,110" fill="currentColor" opacity="0.12" />
        <polyline points={bwLine} fill="none" stroke="currentColor" stroke-width="2" />
      </svg>
      <div class="mt-2 flex justify-between text-xs text-muted-foreground">
        <span>15:15:14</span><span>15:15:34</span><span>15:15:54</span>
      </div>
    </Card>

    <Card title="内存使用" description="平稳在 20% 左右">
      <svg class="h-32 w-full text-primary" viewBox="0 0 300 110" preserveAspectRatio="none">
        <polygon points="0,110 {memLine} 300,110" fill="currentColor" opacity="0.12" />
        <polyline points={memLine} fill="none" stroke="currentColor" stroke-width="2" />
      </svg>
      <div class="mt-2 flex justify-between text-xs text-muted-foreground">
        <span>15:14:14</span><span>15:14:38</span><span>15:15:02</span>
      </div>
    </Card>
  </div>
</div>
