<script lang="ts">
  // 入口：未登录 → 登录页；首登未改密 → 强制改密页；否则 → 管理后台外壳。
  // 旧的频道 demo 面板（ChannelList / EventLog）已从主界面下线，
  // 相关 API 与 SSE 封装仍保留在 lib/，后续需要时可挂回菜单。
  import { clearToken, getToken, setAuthHandlers, setToken } from './lib/api';
  import Login from './lib/components/Login.svelte';
  import ChangePassword from './lib/components/ChangePassword.svelte';
  import AdminShell from './lib/components/AdminShell.svelte';

  let token = $state<string | null>(getToken());
  let mustChange = $state(false);
  let error = $state('');

  setAuthHandlers({
    onUnauthorized: () => {
      token = null;
      mustChange = false;
      error = '登录已失效，请重新登录';
    },
    onPasswordChangeRequired: () => {
      mustChange = true;
    },
  });

  function handleLogin(newToken: string, must: boolean): void {
    setToken(newToken);
    token = newToken;
    mustChange = must;
    error = '';
  }

  function handlePasswordChanged(newToken: string): void {
    setToken(newToken);
    token = newToken;
    mustChange = false;
    error = '';
  }

  function logout(): void {
    clearToken();
    token = null;
    mustChange = false;
    error = '';
  }
</script>

{#if !token}
  <Login onLogin={handleLogin} />
{:else if mustChange}
  <ChangePassword onDone={handlePasswordChanged} onCancel={logout} />
{:else}
  {#if error}<div class="alert" style="margin: 12px 18px">{error}</div>{/if}
  <AdminShell onLogout={logout} />
{/if}
