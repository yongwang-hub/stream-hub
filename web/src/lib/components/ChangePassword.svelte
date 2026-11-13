<script lang="ts">
  import { api } from '../api';

  interface Props {
    onDone: (token: string) => void;
    onCancel: () => void;
  }
  let { onDone, onCancel }: Props = $props();

  let oldPassword = $state('');
  let newPassword = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    error = '';
    if (newPassword.length < 6) {
      error = '新密码至少 6 位';
      return;
    }
    if (newPassword !== confirm) {
      error = '两次输入的新密码不一致';
      return;
    }
    busy = true;
    try {
      const result = await api.changePassword(oldPassword, newPassword);
      onDone(result.token);
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="auth-wrap">
  <div class="card">
    <h2>首次登录 · 修改密码</h2>
    <p class="hint">出于安全要求，必须先设置新密码才能使用控制台。改密后旧 token 会立即失效。</p>

    <div class="note" style="margin-bottom: 16px">
      改密成功后服务端会重签 token，前端自动替换并把 SSE 连接切到新 token。
    </div>

    {#if error}<div class="alert" style="margin-bottom: 14px">{error}</div>{/if}

    <form class="stack" onsubmit={submit}>
      <div class="field">
        <label for="old">当前密码</label>
        <input id="old" type="password" bind:value={oldPassword} autocomplete="current-password" />
      </div>
      <div class="field">
        <label for="new">新密码</label>
        <input id="new" type="password" bind:value={newPassword} autocomplete="new-password" />
      </div>
      <div class="field">
        <label for="confirm">确认新密码</label>
        <input id="confirm" type="password" bind:value={confirm} autocomplete="new-password" />
      </div>
      <div class="inline">
        <button class="btn primary" type="submit" disabled={busy}>
          {busy ? '提交中…' : '保存并进入'}
        </button>
        <button class="btn ghost" type="button" onclick={onCancel}>取消登录</button>
      </div>
    </form>
  </div>
</div>
