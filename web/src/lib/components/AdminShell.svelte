<script lang="ts">
  // 后台布局严格照公司 Vue 框架 packages/shared-components/src/xc-layout/：
  //   xc-header-bar: 60px / padding 0 12px / 与侧栏同色 / logo 55px / 系统名 18px·500·字距1px·text-shadow
  //   xc-side-bar:   220px + 12px×2 = 244px / 菜单项 46px / 文字 14px·500 / 图标 20px / 选中=主色实心
  //   xc-content:    padding 12px / 左上圆角 10px / 上+左边框 / 无阴影
  // 菜单只有系统监控、系统管理两项，切换靠本地 view 状态，不引路由库。
  import defaultLogo from '../../assets/logo.png';
  import Monitor from './Monitor.svelte';
  import Roles from './Roles.svelte';
  import Users from './Users.svelte';
  import Settings from './Settings.svelte';
  import ChangePassword from './ChangePassword.svelte';
  import Icon from '../ui/Icon.svelte';
  import { toast } from '../ui/toast.svelte';
  import { config } from '../systemConfig.svelte';
  import { cn } from '../utils';
  import { currentTheme, toggleThemeWithCircularReveal, type Theme } from '../theme';

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
  let userMenuOpen = $state(false);
  let changePwdOpen = $state(false);

  // 与 Vue 源码 xc-user-popover 一致：主动改密成功后清登录态、回登录页用新密码重新登录，
  // 不直接用后端重签的 token（源码 modify-password 成功后调 onLogout）。
  function handlePasswordChanged(_newToken: string): void {
    changePwdOpen = false;
    onLogout();
  }

  // Vue 源码菜单项带快捷键 Alt+P（修改密码）/ Alt+Q（锁定屏幕）。
  // 本项目未实现锁定屏幕，保留入口但给出提示，避免点击无反应。
  function lockScreen(): void {
    userMenuOpen = false;
    toast.info('锁定屏幕功能暂未实现');
  }

  $effect(() => {
    const onKey = (e: KeyboardEvent): void => {
      if (!e.altKey) return;
      if (e.key.toLowerCase() === 'p') {
        e.preventDefault();
        userMenuOpen = false;
        changePwdOpen = true;
      } else if (e.key.toLowerCase() === 'q') {
        e.preventDefault();
        lockScreen();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

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

  function switchTheme(event: MouseEvent): void {
    theme = toggleThemeWithCircularReveal(event);
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
      {#if config().product_icon}
        <img src={config().product_icon} alt="产品图标" class="h-[55px] w-[55px] shrink-0 rounded-[12px] object-contain" />
      {:else}
        <!-- 未配置自定义图标时展示内置默认 logo（透明底，直接压在顶栏深色背景上） -->
        <img src={defaultLogo} alt="产品图标" class="h-[40px] w-auto shrink-0 object-contain" />
      {/if}
      <span
        class="truncate text-[18px] font-medium tracking-[1px] text-white"
        style="text-shadow: 0 0 1px #fff, 0 0 1px #fff"
      >{config().product_name}</span>
    </div>

    <span class="hidden max-w-[46%] truncate text-[18px] text-warning xl:inline">
      {config().warning_text}
    </span>

    <div class="flex shrink-0 items-center gap-[10px]">
      <button class={topBtn} onclick={toggleFullscreen} title="全屏" aria-label="全屏">
        <Icon name="fullscreen" size={19} />
      </button>
      <button class={topBtn} onclick={switchTheme} title={theme === 'dark' ? '切换浅色' : '切换深色'} aria-label="切换主题">
        <Icon name={theme === 'dark' ? 'sun' : 'moon'} size={19} />
      </button>
      <!-- 头像下拉：照 Vue xc-user-popover 的 hover 弹出样式（280px、无箭头、底部整行退出） -->
      <div
        class="relative"
        role="presentation"
        onmouseenter={() => (userMenuOpen = true)}
        onmouseleave={() => (userMenuOpen = false)}
      >
        <button
          class="grid h-9 w-9 place-items-center rounded-full border-2 border-white/40 overflow-hidden"
          title="用户菜单"
          aria-label="用户菜单"
        >
          <span class="grid h-full w-full place-items-center bg-primary text-[14px] font-medium text-primary-foreground">
            {username.slice(0, 1).toUpperCase()}
          </span>
        </button>

        {#if userMenuOpen}
          <!-- 透明间隙桥接头像与菜单，避免鼠标移到菜单途中触发 mouseleave -->
          <div class="absolute right-0 top-full h-2 w-[280px]"></div>
          <div class="absolute right-0 top-full z-30 mt-2 w-[280px] overflow-hidden rounded-[8px] border border-border bg-card text-card-foreground shadow-xl">
            <div class="flex items-center gap-3 px-4 py-3">
              <span class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-primary text-[16px] font-medium text-primary-foreground">
                {username.slice(0, 1).toUpperCase()}
              </span>
              <span class="min-w-0">
                <span class="block truncate text-[14px] font-semibold">{username}</span>
                <span class="block truncate text-xs text-muted-foreground">{username}@stream-hub.local</span>
              </span>
            </div>
            <div class="border-t border-border"></div>
            <div class="py-1">
              <button
                class="flex h-10 w-full items-center gap-3 px-4 text-left text-[14px] transition-colors hover:bg-muted"
                onclick={() => { userMenuOpen = false; changePwdOpen = true; }}
              >
                <Icon name="password" size={17} />
                <span class="flex-1">修改密码</span>
                <span class="text-xs text-muted-foreground">Alt + P</span>
              </button>
              <button
                class="flex h-10 w-full items-center gap-3 px-4 text-left text-[14px] transition-colors hover:bg-muted"
                onclick={lockScreen}
              >
                <Icon name="lock" size={17} />
                <span class="flex-1">锁定屏幕</span>
                <span class="text-xs text-muted-foreground">Alt + Q</span>
              </button>
            </div>
            <div class="border-t border-border"></div>
            <div class="p-2">
              <button
                class="h-9 w-full rounded-[6px] border border-input bg-background text-[14px] transition-colors hover:bg-muted"
                onclick={onLogout}
              >
                退出登录
              </button>
            </div>
          </div>
        {/if}
      </div>
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

  {#if changePwdOpen}
    <ChangePassword
      mode="active"
      onDone={handlePasswordChanged}
      onCancel={() => (changePwdOpen = false)}
    />
  {/if}
</div>
