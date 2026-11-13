<script lang="ts">
  import { api } from '../api';
  import Icon from '../ui/Icon.svelte';
  import Button from '../ui/Button.svelte';

  interface Props {
    onLogin: (token: string, mustChange: boolean) => void;
  }
  let { onLogin }: Props = $props();

  // “记住我”只记用户名，密码一律不落盘。
  const REMEMBER_KEY = 'sh.login.username';
  let username = $state(localStorage.getItem(REMEMBER_KEY) ?? 'admin');
  let password = $state('');
  let remember = $state(localStorage.getItem(REMEMBER_KEY) !== null);
  let showPassword = $state(false);
  let error = $state('');
  let busy = $state(false);

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      const result = await api.login(username, password);
      if (remember) localStorage.setItem(REMEMBER_KEY, username);
      else localStorage.removeItem(REMEMBER_KEY);
      onLogin(result.token, result.must_change);
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="auth-bg">
  <header class="flex items-center gap-3 p-6 md:px-10">
    <div class="grid h-9 w-9 place-items-center rounded-xl bg-primary text-sm font-bold text-primary-foreground shadow-lg">
      S
    </div>
    <span class="text-base font-bold tracking-wide text-white">stream-hub</span>
    <span class="hidden h-4 w-px bg-white/30 sm:block"></span>
    <span class="hidden text-sm tracking-[0.2em] text-slate-300 sm:block">流媒体分发控制台</span>
  </header>

  <main class="flex flex-1 items-center justify-center px-6 pb-16 md:justify-end md:pr-[10vw]">
    <form
      class="w-full max-w-[380px] rounded-2xl border border-white/15 bg-slate-950/70 p-8 text-slate-100 shadow-2xl backdrop-blur-xl ui-fade-in"
      onsubmit={submit}
    >
      <p class="text-xs font-semibold tracking-[0.25em] text-sky-300/80">STREAM-HUB CONSOLE</p>
      <h1 class="mt-2 text-2xl font-semibold">欢迎登录</h1>
      <p class="mt-1 text-sm text-slate-400">请输入账号密码进入控制台</p>

      {#if error}
        <div class="mt-5 rounded-lg border border-red-400/30 bg-red-500/10 px-3 py-2 text-xs text-red-200">
          {error}
        </div>
      {/if}

      <div class="mt-7 space-y-4">
        <label class="flex items-center gap-3 border-b border-white/20 py-2.5 transition-colors focus-within:border-sky-400">
          <Icon name="user" size={16} class="shrink-0 text-slate-400" />
          <input
            class="w-full bg-transparent text-sm text-slate-100 outline-none placeholder:text-slate-400"
            type="text"
            placeholder="用户名"
            bind:value={username}
            autocomplete="username"
          />
        </label>

        <label class="flex items-center gap-3 border-b border-white/20 py-2.5 transition-colors focus-within:border-sky-400">
          <Icon name="lock" size={16} class="shrink-0 text-slate-400" />
          <input
            class="w-full bg-transparent text-sm text-slate-100 outline-none placeholder:text-slate-400"
            type={showPassword ? 'text' : 'password'}
            placeholder="密码"
            bind:value={password}
            autocomplete="current-password"
          />
          <button
            type="button"
            class="shrink-0 text-slate-400 transition-colors hover:text-white"
            onclick={() => (showPassword = !showPassword)}
            aria-label={showPassword ? '隐藏密码' : '显示密码'}
          >
            <Icon name={showPassword ? 'eye-off' : 'eye'} size={16} />
          </button>
        </label>
      </div>

      <label class="mt-5 flex cursor-pointer select-none items-center gap-2 text-xs text-slate-300">
        <input type="checkbox" class="h-4 w-4 accent-sky-500" bind:checked={remember} />
        记住用户名
      </label>

      <Button type="submit" size="lg" class="mt-7 w-full rounded-full tracking-[0.3em]" disabled={busy}>
        {busy ? '登录中…' : '登 录'}
      </Button>

      <p class="mt-5 text-xs leading-relaxed text-slate-400">
        初始账号 <span class="font-mono text-slate-200">admin / admin</span>，首次登录会要求修改密码。
      </p>
    </form>
  </main>
</div>
