<script lang="ts">
  import type { StreamEvent, StreamStatus } from '../types';

  interface Props {
    events: StreamEvent[];
    status: StreamStatus;
  }
  let { events, status }: Props = $props();

  const labels: Record<StreamStatus, string> = {
    connecting: '连接中',
    open: '已连接',
    closed: '已断开',
  };

  const time = (ts: number): string =>
    new Date(ts).toLocaleTimeString('zh-CN', { hour12: false });
</script>

<section class="card">
  <h2>实时事件流</h2>
  <p class="hint">
    订阅 <code>channels</code> 频道。EventSource 自带断线重连，后端心跳 15s、
    支持 <code>Last-Event-ID</code> 补发。连接状态：{labels[status]}
  </p>

  <div class="log">
    {#each events as event (event.id + '-' + event.receivedAt)}
      <div class="log-item">
        <div class="log-head">
          <span class="log-type">{event.type}</span>
          <span class="log-time">{time(event.receivedAt)} · #{event.id}</span>
        </div>
        <pre class="log-data">{JSON.stringify(event.data)}</pre>
      </div>
    {:else}
      <div class="empty">等待事件…新建频道或点下方按钮推一条</div>
    {/each}
  </div>
</section>
