<script lang="ts">
  import defaultLogo from '../../assets/logo.png';
  import { api } from '../api';
  import { config } from '../systemConfig.svelte';
  import { setCurrentUser } from '../session.svelte';
  import Icon from '../ui/Icon.svelte';
  import { toast } from '../ui/toast.svelte';

  // 登录页样式严格照公司 Vue 项目 packages/shared-views/src/views/xc-login/index.vue：
  //   品牌区 absolute top/left 30px，logo 78px，文字 24px/600/字距3.6px/skewX(-8deg)
  //   表单 460x480 / 圆角10px / padding 20px 30px / 透明无阴影 / 右侧8%
  //   输入框下划线风格（border-bottom 1px #d8d8d8），按钮 40px 高，无验证码行。
  // 复选框文案/行为同源码：记住密码（勾选则连用户名一起存入本地、下次回填）。
  interface Props {
    onLogin: (token: string, mustChange: boolean) => void;
  }
  let { onLogin }: Props = $props();

  // 与 Vue 源码一致：复选框是「记住密码」，勾选时把用户名 + 密码一起存本地并在下次回填。
  // 存储做 Base64 混淆（对齐源码 AES 的“不明文落盘”意图；前端密钥本身不具备保密性）。
  // token 仍只存内存，所以每次打开依然是登录页，只是账号密码已填好。
  const REMEMBER_KEY = 'sh.login.data';
  const encode = (s: string): string =>
    btoa(String.fromCharCode(...new TextEncoder().encode(s)));
  const decode = (s: string): string =>
    new TextDecoder().decode(Uint8Array.from(atob(s), (c) => c.charCodeAt(0)));

  interface SavedLogin {
    username: string;
    password: string;
  }
  localStorage.removeItem('sh.login.username'); // 清理旧版“只记用户名”的残留键。
  let saved: SavedLogin | null = null;
  try {
    const raw = localStorage.getItem(REMEMBER_KEY);
    if (raw) saved = JSON.parse(decode(raw)) as SavedLogin;
  } catch {
    saved = null;
  }
  let username = $state(saved?.username ?? 'admin');
  let password = $state(saved?.password ?? '');
  let remember = $state(saved !== null);
  let showPassword = $state(false);
  let busy = $state(false);

  const describe = (err: unknown): string =>
    err instanceof Error ? err.message : String(err);

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    busy = true;
    try {
      const result = await api.login(username, password);
      if (remember) {
        localStorage.setItem(REMEMBER_KEY, encode(JSON.stringify({ username, password })));
      } else {
        localStorage.removeItem(REMEMBER_KEY);
      }
      toast.success('登录成功');
      setCurrentUser(username); // 水印/顶栏展示当前登录用户名
      onLogin(result.token, result.must_change);
    } catch (err) {
      toast.error(describe(err));
    } finally {
      busy = false;
    }
  }
</script>

<div
  class="auth-bg"
  style={config().login_bg ? `background: url('${config().login_bg}') center / cover no-repeat #0a1022` : ''}
>
  <!-- 品牌区：absolute top/left 30px，78px logo + 2px 分隔线 + 倾斜白色标题 -->
  <div class="absolute left-[30px] top-[30px] flex items-center">
    {#if config().product_icon}
      <img src={config().product_icon} alt="产品图标" class="h-[78px] w-[78px] shrink-0 rounded-[18px] bg-white/90 object-contain shadow-lg" />
    {:else}
      <!-- 未配置自定义图标时展示内置默认 logo（透明底，直接压在登录背景上） -->
      <img src={defaultLogo} alt="产品图标" class="h-[78px] w-auto shrink-0 object-contain" />
    {/if}
    <div class="relative ml-5">
      <span class="absolute -left-[10px] top-1/2 h-[28px] w-[2px] -translate-y-1/2 bg-[#979797]"></span>
      <span
        class="block text-[24px] font-semibold tracking-[3.6px] text-white"
        style="transform: skewX(-8deg)"
      >{config().product_name}</span>
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
      记住密码
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
