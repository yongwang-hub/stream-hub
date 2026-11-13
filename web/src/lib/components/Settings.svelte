<script lang="ts">
  // 系统配置：分组表单骨架。规划契约 GET/PUT /api/v1/config，当前仅本地预览。
  import type { SystemConfig } from '../api';
  import Card from '../ui/Card.svelte';
  import Button from '../ui/Button.svelte';
  import { inputClass, labelClass } from '../ui/classes';

  const defaults: SystemConfig = {
    system_name: 'stream-hub 流媒体分发控制台',
    addr: ':8080',
    log_level: 'info',
    token_ttl: '2h',
    db_driver: 'sqlite',
    db_dsn: 'file:stream-hub.db',
  };

  let config = $state<SystemConfig>({ ...defaults });
  let notice = $state('');

  function save(): void {
    notice = '已在本地预览中保存；后端 GET/PUT /api/v1/config 接口待实现，刷新后不会持久化';
  }

  function reset(): void {
    config = { ...defaults };
    notice = '已恢复默认值';
  }
</script>

<div class="space-y-5">
  <div>
    <h2 class="text-xl font-semibold tracking-tight">系统配置</h2>
    <p class="mt-1 text-sm text-muted-foreground">
      配置表单骨架已就绪。后端接口规划 <code>GET /api/v1/config</code> / <code>PUT /api/v1/config</code>，当前仅本地预览。
    </p>
  </div>

  {#if notice}
    <div class="rounded-lg border border-primary/30 bg-primary/10 px-3 py-2 text-sm text-primary">{notice}</div>
  {/if}

  <Card title="基础配置" description="服务名称、监听地址与日志级别">
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class={labelClass} for="c-name">系统名称</label>
        <input id="c-name" class={inputClass} bind:value={config.system_name} />
      </div>
      <div>
        <label class={labelClass} for="c-addr">监听地址</label>
        <input id="c-addr" class={inputClass} bind:value={config.addr} />
      </div>
      <div>
        <label class={labelClass} for="c-log">日志级别</label>
        <select id="c-log" class={inputClass} bind:value={config.log_level}>
          <option value="debug">debug</option>
          <option value="info">info</option>
          <option value="warn">warn</option>
          <option value="error">error</option>
        </select>
      </div>
    </div>
  </Card>

  <Card title="认证配置" description="Token 有效期与签名密钥">
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class={labelClass} for="c-ttl">Token 有效期</label>
        <input id="c-ttl" class={inputClass} bind:value={config.token_ttl} />
      </div>
      <div>
        <label class={labelClass} for="c-secret">JWT 密钥</label>
        <input id="c-secret" class={inputClass} value="由环境变量 STREAM_HUB_AUTH_SECRET 注入" disabled />
      </div>
    </div>
  </Card>

  <Card title="存储配置" description="数据库驱动与连接串">
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class={labelClass} for="c-driver">数据库驱动</label>
        <select id="c-driver" class={inputClass} bind:value={config.db_driver}>
          <option value="sqlite">sqlite</option>
          <option value="mysql">mysql</option>
          <option value="postgres">postgres</option>
          <option value="memory">memory</option>
        </select>
      </div>
      <div>
        <label class={labelClass} for="c-dsn">连接串 (DSN)</label>
        <input id="c-dsn" class={inputClass} bind:value={config.db_dsn} />
      </div>
    </div>
  </Card>

  <div class="flex gap-2">
    <Button icon="check" onclick={save}>保存配置</Button>
    <Button variant="outline" icon="refresh" onclick={reset}>恢复默认</Button>
  </div>
</div>
