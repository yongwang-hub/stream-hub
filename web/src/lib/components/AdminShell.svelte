<script lang="ts">
  // 主界面外壳：深色顶栏贯通全宽 + 深色侧栏在顶栏下方，两者连成「Γ」形一体骨架；
  // 右侧内容区是一张浅色面板，左上角做内凹圆角，圆弧处露出深色骨架。
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

  const crumbs: Record<View, string> = {
    monitor: '系统监控',
    roles: '系统管理 / 角色管理',
    users: '系统管理 / 用户管理',
    settings: '系统管理 / 系统配置',
  };

  const itemClass = (active: boolean): string =>
    cn(
      'flex w-full items-center gap-3 rounded-lg px-3 py-3 text-sm transition-colors',
      active
        ? 'bg-sidebar-active text-white shadow-sm'
        : 'text-sidebar-muted hover:bg-white/5 hover:text-sidebar-foreground',
    );

  const topBtn =
    'inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-sidebar-muted transition-colors hover:bg-white/10 hover:text-sidebar-foreground';

  function go(next: View): void {
    view = next;
    if (next !== 'monitor') manageOpen = true;
  }

  function switchTheme(): void {
    theme = toggleTheme();
  }
</script>

<div class="flex h-screen flex-col overflow-hidden bg-sidebar">
  <!-- 顶栏：全宽深色，左侧品牌区宽度与侧栏对齐，视觉上与侧栏连成一体 -->
  <header class="flex h-16 shrink-0 items-center gap-3 bg-sidebar px-4 text-sidebar-foreground">
    <div class={cn('flex shrink-0 items-center gap-3', collapsed ? 'w-16 justify-center' : 'w-60')}>
      <div
        class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-primary text-sm font-bold text-primary-foreground shadow-sm"
      >
        S
      </div>
      {#if !collapsed}
        <div class="min-w-0">
          <div class="truncate text-base font-semibold">stream-hub</div>
          <div class="truncate text-xs text-sidebar-muted">流媒体分发控制台</div>
        </div>
      {/if}
    </div>

    <button
      class={topBtn}
      onclick={() => (collapsed = !collapsed)}
      title={collapsed ? '展开菜单' : '收起菜单'}
      aria-label={collapsed ? '展开菜单' : '收起菜单'}
    >
      <Icon name="menu" size={18} />
    </button>

    <span class="text-base font-medium">{crumbs[view]}</span>
    <span class="flex-1"></span>
    <span class="hidden text-xs text-warning lg:inline">
      本系统视频图像严禁拍摄、截屏、录像
    </span>

    <button
      class={topBtn}
      onclick={switchTheme}
      title={theme === 'dark' ? '切换浅色' : '切换深色'}
      aria-label={theme === 'dark' ? '切换浅色' : '切换深色'}
    >
      <Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} />
    </button>

    <span class="hidden items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-1.5 text-sm text-sidebar-muted sm:inline-flex">
      <Icon name="user" size={15} /> {username}
    </span>

    <button
      class="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg border border-white/15 px-3 text-sm text-sidebar-foreground transition-colors hover:bg-white/10"
      onclick={onLogout}
    >
      <Icon name="logout" size={16} /> 退出
    </button>
  </header>

  <div class="flex min-h-0 flex-1">
    <!-- 侧栏：与顶栏同色，从顶栏下方开始，构成深色骨架的竖边 -->
    <aside
      class={cn(
        'ui-scroll flex shrink-0 flex-col overflow-y-auto bg-sidebar text-sidebar-foreground transition-[width] duration-200',
        collapsed ? 'w-16' : 'w-60',
      )}
    >
      <div class="flex-1 px-2.5 pb-4 pt-4">
        {#if !collapsed}
          <div class="px-3 pb-2 text-xs font-semibold tracking-widest text-sidebar-muted">
            功能菜单
          </div>
        {/if}

        <button class={itemClass(view === 'monitor')} onclick={() => go('monitor')} title="系统监控">
          <Icon name="monitor" size={18} />
          {#if !collapsed}<span>系统监控</span>{/if}
        </button>

        <button
          class={cn(itemClass(view !== 'monitor'), 'mt-1')}
          onclick={() => (manageOpen = !manageOpen)}
          title="系统管理"
        >
          <Icon name="settings" size={18} />
          {#if !collapsed}
            <span class="flex-1 text-left">系统管理</span>
            <Icon
              name="chevron-down"
              size={14}
              class={cn('transition-transform', manageOpen ? '' : '-rotate-90')}
            />
          {/if}
        </button>

        {#if manageOpen && !collapsed}
          <div class="ml-5 mt-1 space-y-0.5 border-l border-white/10 pl-3">
            <button class={cn(itemClass(view === 'roles'), 'py-2.5 text-sm')} onclick={() => go('roles')}>
              <Icon name="roles" size={16} /> 角色管理
            </button>
            <button class={cn(itemClass(view === 'users'), 'py-2.5 text-sm')} onclick={() => go('users')}>
              <Icon name="user" size={16} /> 用户管理
            </button>
            <button class={cn(itemClass(view === 'settings'), 'py-2.5 text-sm')} onclick={() => go('settings')}>
              <Icon name="settings" size={16} /> 系统配置
            </button>
          </div>
        {/if}
      </div>

      {#if !collapsed}
        <div class="border-t border-white/10 px-4 py-3 text-xs text-sidebar-muted">
          stream-hub · 演示环境
        </div>
      {/if}
    </aside>

    <!-- 内容面板：左上内凹圆弧，深色骨架从圆角处透出；边框让暗色主题下也能看清圆弧 -->
    <main
      class="ui-scroll min-w-0 flex-1 overflow-auto rounded-tl-[22px] border-l border-t border-border/30 bg-background p-6 text-foreground shadow-sm"
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
