<script lang="ts">
  // 入口：未登录 → 登录页；首登未改密 → 强制改密页；否则 → 管理后台外壳。
  // 全局报错统一走右上角 Toast（Toaster），不再用页面内嵌错误条。
  import { clearToken, getToken, setAuthHandlers, setToken } from './lib/api';
  import { toast } from './lib/ui/toast.svelte';
  import Toaster from './lib/ui/Toaster.svelte';
  import Login from './lib/components/Login.svelte';
  import ChangePassword from './lib/components/ChangePassword.svelte';
  import AdminShell from './lib/components/AdminShell.svelte';

  // token 与 must_change 都只存内存：刷新 / 重开页面一律回到登录页。
  let token = $state<string | null>(getToken());
  let mustChange = $state(false);

  setAuthHandlers({
    onUnauthorized: () => {
      token = null;
      mustChange = false;
      toast.error('登录已失效，请重新登录');
    },
    onPasswordChangeRequired: () => {
      mustChange = true;
    },
  });

  function handleLogin(newToken: string, must: boolean): void {
    setToken(newToken);
    token = newToken;
    mustChange = must;
  }

  // 改密成功后后端会重签 token，直接用新 token 进入主界面。
  function handlePasswordChanged(newToken: string): void {
    setToken(newToken);
    token = newToken;
    mustChange = false;
  }

  function logout(): void {
    clearToken();
    token = null;
    mustChange = false;
  }
</script>

<Toaster />

<!-- 强制改密是叠在登录页之上的 dialog：mustChange 期间登录页必须继续渲染在背景，弹窗用
     fixed inset-0 覆盖。后端登录直接签发了 token，所以这里用 !token || mustChange 判断，
     否则背景会变成后台壳、登录框消失。 -->
{#if !token || mustChange}
  <Login onLogin={handleLogin} />
{:else}
  <AdminShell onLogout={logout} />
{/if}

{#if mustChange}
  <ChangePassword onDone={handlePasswordChanged} onCancel={logout} />
{/if}
