<script lang="ts">
  // 主界面外壳：顶栏 + 左侧菜单（目前只有系统监控、系统管理两项）+ 右侧内容区。
  // 不引入路由库：菜单切换就是一个本地 view 状态，后续接真实路由也很容易替换。
  import Monitor from './Monitor.svelte';
  import Roles from './Roles.svelte';
  import Users from './Users.svelte';
  import Settings from './Settings.svelte';

  type View = 'monitor' | 'roles' | 'users' | 'settings';

  interface Props {
    onLogout: () => void;
    username?: string;
  }
  let { onLogout, username = 'admin' }: Props = $props();

  let view = $state<View>('monitor');
  let manageOpen = $state(true);

  const crumbs: Record<View, string> = {
    monitor: '系统监控',
    roles: '系统管理 / 角色管理',
    users: '系统管理 / 用户管理',
    settings: '系统管理 / 系统配置',
  };

  function go(next: View): void {
    view = next;
    if (next !== 'monitor') manageOpen = true;
  }
</script>

<div class="admin">
  <aside class="admin-side">
    <div class="side-brand">
      <div class="brand-mark">S</div>
      <div>
        <strong>stream-hub</strong>
        <small>流媒体分发控制台</small>
      </div>
    </div>

    <div class="side-label">功能菜单</div>
    <button
      class="menu-item {view === 'monitor' ? 'active' : ''}"
      onclick={() => go('monitor')}
    >
      <span class="mi">▦</span> 系统监控
    </button>

    <button
      class="menu-item {view !== 'monitor' ? 'active-parent' : ''}"
      onclick={() => (manageOpen = !manageOpen)}
    >
      <span class="mi">⚙</span> 系统管理
      <span class="chev">{manageOpen ? '⌄' : '›'}</span>
    </button>
    {#if manageOpen}
      <div class="submenu">
        <button class="menu-item {view === 'roles' ? 'active' : ''}" onclick={() => go('roles')}>角色管理</button>
        <button class="menu-item {view === 'users' ? 'active' : ''}" onclick={() => go('users')}>用户管理</button>
        <button class="menu-item {view === 'settings' ? 'active' : ''}" onclick={() => go('settings')}>系统配置</button>
      </div>
    {/if}

    <div class="side-foot">stream-hub · 演示环境</div>
  </aside>

  <div class="admin-main">
    <header class="admin-top">
      <span class="title">{crumbs[view]}</span>
      <span class="spacer"></span>
      <span class="top-note">本系统视频图像严禁拍摄、截屏、录像</span>
      <span class="user-chip">◉ {username}</span>
      <button class="btn ghost sm" onclick={onLogout}>退出登录</button>
    </header>

    <main class="admin-content">
      {#if view === 'monitor'}
        <Monitor />
      {:else if view === 'roles'}
        <Roles />
      {:else if view === 'users'}
        <Users />
      {:else}
        <Settings />
      {/if}
    </main>
  </div>
</div>
