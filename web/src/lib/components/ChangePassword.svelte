<script lang="ts">
  import { api } from '../api';
  import Icon from '../ui/Icon.svelte';
  import Button from '../ui/Button.svelte';
  import { labelClass } from '../ui/classes';

  interface Props {
    onDone: (token: string) => void;
    onCancel: () => void;
  }
  let { onDone, onCancel }: Props = $props();

  // 深色玻璃卡片上的输入框，单独定义以免与浅色主题的 inputClass 冲突。
  const darkInput =
    'w-full rounded-lg border border-white/15 bg-slate-900/60 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-400 transition-colors focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/20';

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

<div class="auth-bg">
  <main class="flex flex-1 items-center justify-center px-6 pb-10">
    <form
      class="w-full max-w-[420px] rounded-2xl border border-white/15 bg-slate-950/75 p-8 text-slate-100 shadow-2xl backdrop-blur-xl ui-fade-in"
      onsubmit={submit}
    >
      <div class="mb-6 flex items-center gap-3">
        <div class="grid h-10 w-10 place-items-center rounded-xl bg-primary text-primary-foreground">
          <Icon name="lock" size={18} />
        </div>
        <div>
          <h2 class="text-lg font-semibold">首次登录 · 修改密码</h2>
          <p class="text-xs text-slate-400">设置新密码后才能使用控制台</p>
        </div>
      </div>

      <div class="rounded-lg border border-sky-400/20 bg-sky-400/10 px-3 py-2 text-xs text-sky-200">
        改密成功后服务端会重签 token，旧 token 立即失效。
      </div>

      {#if error}
        <div class="mt-4 rounded-lg border border-red-400/30 bg-red-500/10 px-3 py-2 text-xs text-red-200">
          {error}
        </div>
      {/if}

      <div class="mt-5 space-y-4">
        <div>
          <label class={labelClass} for="old">当前密码</label>
          <input id="old" class={darkInput} type="password" bind:value={oldPassword} autocomplete="current-password" />
        </div>
        <div>
          <label class={labelClass} for="new">新密码</label>
          <input id="new" class={darkInput} type="password" bind:value={newPassword} autocomplete="new-password" placeholder="至少 6 位" />
        </div>
        <div>
          <label class={labelClass} for="confirm">确认新密码</label>
          <input id="confirm" class={darkInput} type="password" bind:value={confirm} autocomplete="new-password" />
        </div>
      </div>

      <div class="mt-6 flex gap-2">
        <Button type="submit" class="flex-1" disabled={busy}>
          {busy ? '提交中…' : '保存并进入'}
        </Button>
        <Button variant="outline" onclick={onCancel}>取消登录</Button>
      </div>
    </form>
  </main>
</div>
