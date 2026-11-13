<script lang="ts">
  // 系统监控：本轮先做静态示意（死 HTML），不接后端、不发请求。
  // 后续接真实指标时，把这些常量换成 api 返回的时序数据即可。
  const loads = [
    { label: '直播', value: 0 },
    { label: '回放', value: 0 },
    { label: '播放', value: 0 },
    { label: '录像', value: 0 },
    { label: 'H265', value: 0 },
    { label: '级联', value: 0 },
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

<h2 class="page-title">系统监控</h2>
<p class="page-hint">概览页面 · 当前为静态示意数据（未接后端指标接口）</p>

<div class="monitor-grid">
  <section class="m-card">
    <h3>通道统计</h3>
    <div class="ring-row">
      <div class="ring-box">
        <div class="ring">100%</div>
        <small>在线率: 1</small>
      </div>
      <div class="ring-box">
        <div class="ring">100%</div>
        <small>在线数: 58</small>
      </div>
    </div>
  </section>

  <section class="m-card">
    <h3>负载统计</h3>
    <div class="bars">
      {#each loads as item}
        <div class="bar-col">
          <div class="bar"></div>
          {item.value}<br />{item.label}
        </div>
      {/each}
    </div>
  </section>

  <section class="m-card">
    <h3>CPU 使用</h3>
    <svg class="chart" viewBox="0 0 300 110" preserveAspectRatio="none">
      <polyline points={cpuLine} fill="none" stroke="#5b8cff" stroke-width="2" />
      <polygon points="0,110 {cpuLine} 300,110" fill="#5b8cff22" />
    </svg>
    <div class="axis"><span>15:14:14</span><span>15:14:38</span><span>15:15:02</span></div>
  </section>

  <section class="m-card">
    <h3>存储使用</h3>
    <div class="prog-list">
      {#each disks as disk}
        <div class="prog-row">
          <span>{disk.name}</span>
          <div class="prog"><span style="width: {disk.used}%"></span></div>
          <span>{disk.used}%</span>
        </div>
      {/each}
    </div>
  </section>

  <section class="m-card">
    <h3>带宽使用 (Mbps)</h3>
    <svg class="chart" viewBox="0 0 300 110" preserveAspectRatio="none">
      <polyline points={bwLine} fill="none" stroke="#5b8cff" stroke-width="2" />
      <polygon points="0,110 {bwLine} 300,110" fill="#5b8cff22" />
    </svg>
    <div class="axis"><span>15:15:14</span><span>15:15:34</span><span>15:15:54</span></div>
  </section>

  <section class="m-card">
    <h3>内存使用</h3>
    <svg class="chart" viewBox="0 0 300 110" preserveAspectRatio="none">
      <polyline points={memLine} fill="none" stroke="#5b8cff" stroke-width="2" />
      <polygon points="0,110 {memLine} 300,110" fill="#5b8cff22" />
    </svg>
    <div class="axis"><span>15:14:14</span><span>15:14:38</span><span>15:15:02</span></div>
  </section>
</div>
