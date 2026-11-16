<script lang="ts">
  // 后台布局严格照公司 Vue 框架 packages/shared-components/src/xc-layout/：
  //   xc-header-bar: 60px / padding 0 12px / 与侧栏同色 / logo 55px / 系统名 18px·500·字距1px·text-shadow
  //   xc-side-bar:   220px + 12px×2 = 244px / 菜单项 46px / 文字 14px·500 / 图标 20px / 选中=主色实心
  //   xc-content:    padding 12px / 左上圆角 10px / 上+左边框 / 无阴影
  // 菜单只有系统监控、系统管理两项，切换靠本地 view 状态，不引路由库。
  import Monitor from './Monitor.svelte';
  import Roles from './Roles.svelte';
  import Users from './Users.svelte';
  import Settings from './Settings.svelte';
  import Icon from '../ui/Icon.svelte';
  import { cn } from '../utils';
  import { currentTheme, toggleTheme, type Theme } from '../theme';

  type View = 'monitor' | 'roles' | 'users' | 'settings';

  interface Props {
    onLogout: () => void;
    username?: string;
  }
  let { onLogout, username = 'admin' }: Props = $props();

  let view = $state<View>('monitor');
  let manageOpen = $state(true);
  let collapsed = $state(false);
  let theme = $state<Theme>(currentTheme());

  const itemClass = (active: boolean, isCollapsed = false): string =>
    cn(
      'group flex h-[46px] w-full items-center gap-3 rounded-lg text-[14px] font-medium transition-colors',
      isCollapsed ? 'justify-center px-0' : 'px-3',
      active ? 'bg-primary text-white' : 'text-sidebar-foreground hover:bg-white/10',
    );

  // 顶栏动作按钮：38×38 / 6px 圆角 / 19px 图标（xc-header-bar L107-126）
  const topBtn =
    'inline-flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-[6px] text-sidebar-foreground transition-colors hover:bg-white/10';
  const iconHover = 'transition-transform group-hover:scale-[1.16]';

  function go(next: View): void {
    view = next;
    if (next !== 'monitor') manageOpen = true;
  }

  function switchTheme(): void {
    theme = toggleTheme();
  }

  function toggleFullscreen(): void {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void document.documentElement.requestFullscreen();
  }
</script>

<div class="flex h-screen flex-col overflow-hidden bg-sidebar">
  <!-- 顶栏：60px，与侧栏同色、无下边框；logo 55px + 系统名 18px·500·字距1px -->
  <header class="flex h-[60px] shrink-0 items-center justify-between bg-sidebar px-3 text-sidebar-foreground">
    <div class="flex min-w-0 items-center gap-3">
      <div class="grid h-[55px] w-[55px] shrink-0 place-items-center rounded-[12px] bg-primary text-[24px] font-bold text-primary-foreground">
        S
      </div>
      <span
        class="truncate text-[18px] font-medium tracking-[1px] text-white"
        style="text-shadow: 0 0 1px #fff, 0 0 1px #fff"
      >流媒体输出分发平台</span>
    </div>

    <span class="hidden text-[18px] text-warning xl:inline">
      本系统视频图像严禁拍摄、截屏、录像
    </span>

    <div class="flex shrink-0 items-center gap-[10px]">
      <button class={topBtn} onclick={toggleFullscreen} title="全屏" aria-label="全屏">
        <Icon name="fullscreen" size={19} />
      </button>
      <button class={topBtn} onclick={switchTheme} title={theme === 'dark' ? '切换浅色' : '切换深色'} aria-label="切换主题">
        <Icon name={theme === 'dark' ? 'sun' : 'moon'} size={19} />
      </button>
      <span class="hidden items-center gap-2 sm:flex">
        <span class="grid h-8 w-8 place-items-center rounded-full border-2 border-border bg-card text-[14px] font-medium text-foreground">
          {username.slice(0, 1).toUpperCase()}
        </span>
        <span class="text-[14px]">{username}</span>
      </span>
      <button class={topBtn} onclick={onLogout} title="退出登录" aria-label="退出登录">
        <Icon name="logout" size={19} />
      </button>
    </div>
  </header>

  <div class="flex min-h-0 flex-1">
    <!-- 侧栏：244px / padding 0 12px；菜单项 46px 高、14px 字、20px 图标、选中主色实心 -->
    <aside
      class={cn(
        'ui-scroll flex shrink-0 flex-col overflow-y-auto bg-sidebar text-sidebar-foreground transition-[width] duration-200',
        collapsed ? 'w-[88px]' : 'w-[244px]',
      )}
    >
      <div class="flex-1 px-3 pb-4 pt-4">
        {#if !collapsed}
          <div class="px-3 pb-2 text-[14px] font-medium text-sidebar-muted">功能菜单</div>
        {/if}

        <button class={itemClass(view === 'monitor', collapsed)} onclick={() => go('monitor')} title="系统监控">
          <Icon name="monitor" size={20} class={iconHover} />
          {#if !collapsed}<span>系统监控</span>{/if}
        </button>

        <button
          class={cn(itemClass(view !== 'monitor', collapsed), 'mt-1')}
          onclick={() => (manageOpen = !manageOpen)}
          title="系统管理"
        >
          <Icon name="settings" size={20} class={iconHover} />
          {#if !collapsed}
            <span class="flex-1 text-left">系统管理</span>
            <Icon
              name="chevron-down"
              size={16}
              class={cn('transition-transform', manageOpen ? '' : '-rotate-90')}
            />
          {/if}
        </button>

        {#if manageOpen && !collapsed}
          <div class="ml-5 mt-1 space-y-0.5 border-l border-white/10 pl-3">
            <button class={itemClass(view === 'roles')} onclick={() => go('roles')}>
              <Icon name="roles" size={19} class={iconHover} /> 角色管理
            </button>
            <button class={itemClass(view === 'users')} onclick={() => go('users')}>
              <Icon name="user" size={19} class={iconHover} /> 用户管理
            </button>
            <button class={itemClass(view === 'settings')} onclick={() => go('settings')}>
              <Icon name="settings" size={19} class={iconHover} /> 系统配置
            </button>
          </div>
        {/if}
      </div>

      <!-- 折叠按钮：26×26 / 1px 边框 / 10px 圆角（xc-side-bar L159-182） -->
      <div class="flex justify-center border-t border-white/10 py-3">
        <button
          class="grid h-[26px] w-[26px] place-items-center rounded-[10px] border border-border bg-background text-foreground transition-colors hover:opacity-80"
          onclick={() => (collapsed = !collapsed)}
          title={collapsed ? '展开菜单' : '收起菜单'}
          aria-label={collapsed ? '展开菜单' : '收起菜单'}
        >
          <Icon name="menu" size={14} />
        </button>
      </div>
    </aside>

    <!-- 内容区：padding 12px / 左上圆角 10px / 上+左边框 / 无阴影 -->
    <main
      class="ui-scroll min-w-0 flex-1 overflow-auto rounded-tl-[10px] border-l border-t border-border bg-background p-3 text-foreground"
    >
      <div class="mx-auto max-w-[1440px] ui-fade-in">
        {#if view === 'monitor'}
          <Monitor />
        {:else if view === 'roles'}
          <Roles />
        {:else if view === 'users'}
          <Users />
        {:else}
          <Settings />
        {/if}
      </div>
    </main>
  </div>
</div>
