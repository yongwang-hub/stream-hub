<script lang="ts">
  import { api } from '../api';
  import Icon from '../ui/Icon.svelte';
  import { toast } from '../ui/toast.svelte';

  // 登录页样式严格照公司 Vue 项目 packages/shared-views/src/views/xc-login/index.vue：
  //   品牌区 absolute top/left 30px，logo 78px，文字 24px/600/字距3.6px/skewX(-8deg)
  //   表单 460x480 / 圆角10px / padding 20px 30px / 透明无阴影 / 右侧8%
  //   输入框下划线风格（border-bottom 1px #d8d8d8），按钮 40px 高，无验证码行。
  interface Props {
    onLogin: (token: string, mustChange: boolean) => void;
  }
  let { onLogin }: Props = $props();

  const REMEMBER_KEY = 'sh.login.username';
  let username = $state(localStorage.getItem(REMEMBER_KEY) ?? 'admin');
  let password = $state('');
  let remember = $state(localStorage.getItem(REMEMBER_KEY) !== null);
  let showPassword = $state(false);
  let busy = $state(false);

  const describe = (err: unknown): string =>
    err instanceof Error ? err.message : String(err);

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    busy = true;
    try {
      const result = await api.login(username, password);
      if (remember) localStorage.setItem(REMEMBER_KEY, username);
      else localStorage.removeItem(REMEMBER_KEY);
      toast.success('登录成功');
      onLogin(result.token, result.must_change);
    } catch (err) {
      toast.error(describe(err));
    } finally {
      busy = false;
    }
  }
</script>

<div class="auth-bg">
  <!-- 品牌区：absolute top/left 30px，78px logo + 2px 分隔线 + 倾斜白色标题 -->
  <div class="absolute left-[30px] top-[30px] flex items-center">
    <span
      class="grid h-[78px] w-[78px] shrink-0 place-items-center rounded-[18px] bg-primary text-[32px] font-bold text-primary-foreground shadow-lg"
    >
      S
    </span>
    <div class="relative ml-5">
      <span class="absolute -left-[10px] top-1/2 h-[28px] w-[2px] -translate-y-1/2 bg-[#979797]"></span>
      <span
        class="block text-[24px] font-semibold tracking-[3.6px] text-white"
        style="transform: skewX(-8deg)"
      >流媒体输出分发平台</span>
    </div>
  </div>

  <!-- 登录表单：460x480、透明、右侧 8%、垂直居中偏上 2% -->
  <form
    class="fixed right-[8%] top-1/2 flex h-[480px] w-[460px] max-w-[calc(100vw-40px)] -translate-y-[52%] flex-col justify-center rounded-[10px] px-[30px] py-5 text-white"
    onsubmit={submit}
  >
    <h1 class="my-5 text-[24px] font-medium text-white" style="text-indent: 12px">欢迎登录，</h1>

    <div class="space-y-[18px]">
      <label class="flex h-10 items-center gap-3 border-b border-[#d8d8d8] px-[11px] transition-colors focus-within:border-primary">
        <Icon name="user" size={16} class="shrink-0 text-[#c2bebe]" />
        <input
          class="w-full bg-transparent text-[14px] text-[#c2bebe] outline-none placeholder:text-[#c2bebe]"
          type="text"
          placeholder="用户名"
          bind:value={username}
          autocomplete="username"
        />
      </label>

      <label class="flex h-10 items-center gap-3 border-b border-[#d8d8d8] px-[11px] transition-colors focus-within:border-primary">
        <Icon name="lock" size={16} class="shrink-0 text-[#c2bebe]" />
        <input
          class="w-full bg-transparent text-[14px] text-[#c2bebe] outline-none placeholder:text-[#c2bebe]"
          type={showPassword ? 'text' : 'password'}
          placeholder="密码"
          bind:value={password}
          autocomplete="current-password"
        />
        <button
          type="button"
          class="shrink-0 text-[#c2bebe] transition-colors hover:text-white"
          onclick={() => (showPassword = !showPassword)}
          aria-label={showPassword ? '隐藏密码' : '显示密码'}
        >
          <Icon name={showPassword ? 'eye-off' : 'eye'} size={16} />
        </button>
      </label>
    </div>

    <label class="ml-2 mt-5 flex cursor-pointer select-none items-center gap-2 text-[14px] text-white">
      <input type="checkbox" class="h-4 w-4 accent-primary" bind:checked={remember} />
      记住用户名
    </label>

    <button
      type="submit"
      class="mt-5 h-10 w-full rounded-[10px] bg-primary text-[14px] font-medium text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-60"
      disabled={busy}
    >
      {busy ? '登录中…' : '登录'}
    </button>
  </form>
</div>
