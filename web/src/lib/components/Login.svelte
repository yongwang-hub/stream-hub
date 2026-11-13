<script lang="ts">
  import { api } from '../api';

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

<div class="login">
  <header class="login-brand">
    <div class="brand-mark">S</div>
    <span class="brand-name">stream-hub</span>
    <span class="brand-divider"></span>
    <span class="brand-sub">流媒体分发控制台</span>
  </header>

  <main class="login-panel">
    <form class="login-form" onsubmit={submit}>
      <h1>欢迎登录，</h1>

      {#if error}<div class="alert">{error}</div>{/if}

      <label class="line">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <circle cx="12" cy="8" r="3.6" />
          <path d="M5 20c.8-3.4 3.6-5 7-5s6.2 1.6 7 5" />
        </svg>
        <input type="text" placeholder="用户名" bind:value={username} autocomplete="username" />
      </label>

      <label class="line">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="5" y="10.5" width="14" height="9.5" rx="2" />
          <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" />
        </svg>
        <input
          type={showPassword ? 'text' : 'password'}
          placeholder="密码"
          bind:value={password}
          autocomplete="current-password"
        />
        <button
          type="button"
          class="eye"
          onclick={() => (showPassword = !showPassword)}
          aria-label={showPassword ? '隐藏密码' : '显示密码'}
        >
          {#if showPassword}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 3l18 18" />
              <path d="M10.6 5.8A9.8 9.8 0 0 1 12 5.5c6 0 9.5 6.5 9.5 6.5a17.5 17.5 0 0 1-3 3.9M6.1 6.1A17 17 0 0 0 2.5 12S6 18.5 12 18.5c1.4 0 2.8-.3 4-.9" />
              <path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" />
            </svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
          {/if}
        </button>
      </label>

      <label class="remember">
        <input type="checkbox" bind:checked={remember} />
        <span>记住用户名</span>
      </label>

      <button class="submit" type="submit" disabled={busy}>
        {busy ? '登录中…' : '登 录'}
      </button>

      <p class="hint">
        初始密码由 <code>STREAM_HUB_ADMIN_PASSWORD</code> 注入，未设置时见后端启动日志；首次登录需先改密。
      </p>
    </form>
  </main>
</div>

<style>
  /* 底图在 src/assets/login-bg.png，由 Vite 打包进 /assets/；缺失时由下方深色渐变兜底。 */
  .login {
    position: fixed;
    inset: 0;
    display: flex;
    flex-direction: column;
    background:
      linear-gradient(90deg, rgba(5, 10, 26, 0.5) 0%, rgba(5, 10, 26, 0.18) 42%, rgba(5, 10, 26, 0.6) 100%),
      url('../../assets/login-bg.png') center / cover no-repeat,
      radial-gradient(1100px 620px at 20% 32%, #16305e, transparent),
      linear-gradient(160deg, #0a1022, #0b1020);
    color: var(--text);
  }

  .login-brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 20px 28px;
  }
  .login-brand .brand-mark {
    width: 34px;
    height: 34px;
    border-radius: 10px;
    font-size: 15px;
  }
  .brand-name {
    font-size: 17px;
    font-weight: 800;
    letter-spacing: 0.5px;
  }
  .brand-divider {
    width: 1px;
    height: 16px;
    background: rgba(255, 255, 255, 0.28);
  }
  .brand-sub {
    font-size: 14px;
    letter-spacing: 2px;
    color: #cdd9f2;
  }

  .login-panel {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: 0 11vw 7vh 0;
  }
  .login-form {
    width: min(340px, 84vw);
  }
  h1 {
    margin: 0 0 26px;
    font-size: 26px;
    font-weight: 600;
    letter-spacing: 1px;
  }

  /* 下划线式输入行 */
  .line {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 2px;
    margin-bottom: 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.18);
    transition: border-color 0.15s;
  }
  .line:focus-within {
    border-color: #4d8dff;
  }
  .line > svg {
    flex: none;
    width: 16px;
    height: 16px;
    color: #9fb3d9;
  }
  .line input {
    flex: 1;
    min-width: 0;
    padding: 0;
    background: transparent;
    border: none;
    border-radius: 0;
    box-shadow: none;
    color: var(--text);
    font-size: 14px;
    outline: none;
  }
  .line input::placeholder {
    color: #7e8db0;
  }
  .line input:focus {
    border: none;
    box-shadow: none;
  }

  .eye {
    flex: none;
    display: grid;
    place-items: center;
    padding: 2px;
    background: none;
    border: none;
    color: #9fb3d9;
    cursor: pointer;
  }
  .eye svg {
    width: 16px;
    height: 16px;
  }

  .remember {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 2px 0 26px;
    font-size: 13px;
    color: #b7c4e0;
    cursor: pointer;
    user-select: none;
  }
  .remember input {
    width: 15px;
    height: 15px;
    padding: 0;
    accent-color: #2f7bff;
    cursor: pointer;
  }

  .submit {
    width: 100%;
    padding: 12px;
    border: none;
    border-radius: 22px;
    background: linear-gradient(90deg, #2f7bff, #4d8dff);
    color: #fff;
    font-size: 14px;
    font-weight: 600;
    letter-spacing: 8px;
    text-indent: 8px; /* 平衡 letter-spacing 造成的右侧空隙 */
    cursor: pointer;
    transition: filter 0.15s, transform 0.12s;
  }
  .submit:hover:not(:disabled) {
    filter: brightness(1.12);
    transform: translateY(-1px);
  }
  .submit:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .hint {
    margin-top: 22px;
    font-size: 12px;
    line-height: 1.7;
    color: #7e8db0;
  }
  .hint code {
    color: #a9bcdf;
  }

  @media (max-width: 760px) {
    .login-panel {
      justify-content: center;
      padding: 0 24px;
    }
    .brand-sub,
    .brand-divider {
      display: none;
    }
  }
</style>
