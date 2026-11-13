<script lang="ts">
  // 系统配置：后端暂未提供接口，本轮先做分组表单骨架 + 本地状态。
  // 规划契约：GET /api/v1/config 读取、PUT /api/v1/config 保存（整体覆盖或按分组 PATCH）。
  // 后端补上后，把 config 初值换成 GET 结果、save() 换成 PUT 请求即可。
  import type { SystemConfig } from '../api';

  let config = $state<SystemConfig>({
    system_name: 'stream-hub 流媒体分发控制台',
    addr: ':8080',
    log_level: 'info',
    token_ttl: '2h',
    db_driver: 'sqlite',
    db_dsn: 'file:stream-hub.db',
  });
  let notice = $state('');

  function save(): void {
    notice = '已在本地预览中保存；后端 GET/PUT /api/v1/config 接口待实现，刷新后不会持久化';
  }

  function reset(): void {
    config = {
      system_name: 'stream-hub 流媒体分发控制台',
      addr: ':8080',
      log_level: 'info',
      token_ttl: '2h',
      db_driver: 'sqlite',
      db_dsn: 'file:stream-hub.db',
    };
    notice = '已恢复默认值';
  }
</script>

<h2 class="page-title">系统配置</h2>
<p class="page-hint">
  配置表单骨架已就绪。后端接口规划：<code>GET /api/v1/config</code> 读取、
  <code>PUT /api/v1/config</code> 保存；当前仅本地预览，不会写回服务端。
</p>

{#if notice}<div class="note" style="margin-bottom: 12px">{notice}</div>{/if}

<div class="panel">
  <h3 style="font-size: 13px; margin: 0 0 14px">基础配置</h3>
  <div class="form-grid">
    <div class="field">
      <label for="c-name">系统名称</label>
      <input id="c-name" bind:value={config.system_name} />
    </div>
    <div class="field">
      <label for="c-addr">监听地址</label>
      <input id="c-addr" bind:value={config.addr} />
    </div>
    <div class="field">
      <label for="c-log">日志级别</label>
      <select id="c-log" bind:value={config.log_level}>
        <option value="debug">debug</option>
        <option value="info">info</option>
        <option value="warn">warn</option>
        <option value="error">error</option>
      </select>
    </div>
  </div>
</div>

<div class="panel">
  <h3 style="font-size: 13px; margin: 0 0 14px">认证配置</h3>
  <div class="form-grid">
    <div class="field">
      <label for="c-ttl">Token 有效期</label>
      <input id="c-ttl" bind:value={config.token_ttl} />
    </div>
    <div class="field">
      <label for="c-secret">JWT 密钥</label>
      <input id="c-secret" value="由环境变量 STREAM_HUB_AUTH_SECRET 注入" disabled />
    </div>
  </div>
</div>

<div class="panel">
  <h3 style="font-size: 13px; margin: 0 0 14px">存储配置</h3>
  <div class="form-grid">
    <div class="field">
      <label for="c-driver">数据库驱动</label>
      <select id="c-driver" bind:value={config.db_driver}>
        <option value="sqlite">sqlite</option>
        <option value="mysql">mysql</option>
        <option value="postgres">postgres</option>
        <option value="memory">memory</option>
      </select>
    </div>
    <div class="field">
      <label for="c-dsn">连接串 (DSN)</label>
      <input id="c-dsn" bind:value={config.db_dsn} />
    </div>
  </div>
</div>

<div class="toolbar" style="margin-top: 16px; margin-bottom: 0">
  <button class="btn primary" onclick={save}>保存配置</button>
  <button class="btn ghost" onclick={reset}>恢复默认</button>
</div>
