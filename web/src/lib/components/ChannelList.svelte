<script lang="ts">
  import type { Channel, ChannelInput, ChannelStatus } from '../types';

  interface Props {
    channels: Channel[];
    busy: boolean;
    onCreate: (input: ChannelInput) => void;
    onStatus: (channel: Channel, status: ChannelStatus) => void;
    onDelete: (channel: Channel) => void;
  }
  let { channels, busy, onCreate, onStatus, onDelete }: Props = $props();

  let slug = $state('');
  let name = $state('');
  let sourceUrl = $state('');

  function submit(event: SubmitEvent): void {
    event.preventDefault();
    if (!slug.trim() || !name.trim() || !sourceUrl.trim()) return;
    onCreate({ slug: slug.trim(), name: name.trim(), source_url: sourceUrl.trim() });
    slug = '';
    name = '';
    sourceUrl = '';
  }
</script>

<section class="card">
  <h2>频道</h2>
  <p class="hint">
    新建、改状态、删除都会通过 SSE 广播给所有已打开的页面，列表无需手动刷新。
  </p>

  <form class="stack" onsubmit={submit}>
    <div class="form-row">
      <div class="field">
        <label for="slug">slug</label>
        <input id="slug" bind:value={slug} placeholder="lobby" />
      </div>
      <div class="field">
        <label for="cname">名称</label>
        <input id="cname" bind:value={name} placeholder="大堂摄像头" />
      </div>
    </div>
    <div class="field">
      <label for="source">媒体源 URL</label>
      <input id="source" bind:value={sourceUrl} placeholder="https://example.com/live.m3u8" />
    </div>
    <div>
      <button class="btn primary" type="submit" disabled={busy}>新建频道</button>
    </div>
  </form>

  <div style="margin-top: 22px">
    {#if channels.length === 0}
      <div class="empty">还没有频道，先建一个吧</div>
    {:else}
      <table>
        <thead>
          <tr>
            <th>名称 / slug</th>
            <th>状态</th>
            <th>媒体源</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each channels as channel (channel.id)}
            <tr>
              <td>
                <div>{channel.name}</div>
                <div class="mono" style="font-size: 11px; color: var(--muted)">{channel.slug}</div>
              </td>
              <td><span class="badge {channel.status}">{channel.status}</span></td>
              <td class="mono" style="max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">
                {channel.source_url}
              </td>
              <td class="actions">
                {#if channel.status === 'live'}
                  <button class="btn sm" onclick={() => onStatus(channel, 'stopped')}>停止</button>
                {:else}
                  <button class="btn sm" onclick={() => onStatus(channel, 'live')}>上线</button>
                {/if}
                <button class="btn sm danger" onclick={() => onDelete(channel)}>删除</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</section>
