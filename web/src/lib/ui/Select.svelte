<script lang="ts">
  // 自定义下拉框：原生 <select> 的展开面板由系统绘制（Windows 上是灰底系统菜单），样式无法定制，
  // 这里按公司 Vue el-select 的观感自绘：白底描边触发区 + 圆角浮层 + 浅灰悬停/选中项 + 上方小三角。
  // 触发区不带箭头，宽度完全跟随外部容器，浮层与触发区同宽。
  // 用法：<Select bind:value options={[{ value, label }]} class="w-[220px]" />
  import { cn } from '../utils';

  interface Option {
    value: string;
    label: string;
    disabled?: boolean;
  }

  interface Props {
    value?: string;
    options: Option[];
    placeholder?: string;
    disabled?: boolean;
    class?: string;
    onchange?: (value: string) => void;
  }

  let {
    value = $bindable(''),
    options,
    placeholder = '请选择',
    disabled = false,
    class: className = '',
    onchange,
  }: Props = $props();

  let open = $state(false);
  let activeIndex = $state(-1);
  let trigger = $state<HTMLButtonElement | null>(null);
  let panel = $state<HTMLDivElement | null>(null);

  const selected = $derived(options.find((o) => o.value === value) ?? null);

  function openMenu(): void {
    if (disabled) return;
    open = true;
    const i = options.findIndex((o) => o.value === value);
    activeIndex = i >= 0 ? i : 0;
  }

  function close(focusTrigger = false): void {
    open = false;
    if (focusTrigger) trigger?.focus();
  }

  function pick(option: Option): void {
    if (option.disabled) return;
    value = option.value;
    onchange?.(option.value);
    close(true);
  }

  function onKeydown(event: KeyboardEvent): void {
    if (disabled) return;
    if (!open) {
      if (['Enter', ' ', 'ArrowDown', 'ArrowUp'].includes(event.key)) {
        event.preventDefault();
        openMenu();
      }
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      close(true);
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const step = event.key === 'ArrowDown' ? 1 : -1;
      activeIndex = (activeIndex + step + options.length) % options.length;
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const option = options[activeIndex];
      if (option) pick(option);
    } else if (event.key === 'Tab') {
      close();
    }
  }

  // 点到组件外面就收起（用 pointerdown，避免和按钮自身的 click 抢事件）
  $effect(() => {
    const onPointerDown = (event: PointerEvent): void => {
      if (!open) return;
      const target = event.target as Node;
      if (panel?.contains(target) || trigger?.contains(target)) return;
      close();
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => document.removeEventListener('pointerdown', onPointerDown);
  });
</script>

<div class={cn('relative w-full', className)}>
  <button
    bind:this={trigger}
    type="button"
    {disabled}
    class={cn(
      'flex w-full items-center rounded-lg border bg-card px-3 py-2 text-left text-sm transition-colors',
      disabled && 'cursor-not-allowed opacity-60',
      open ? 'border-ring ring-2 ring-ring/20' : 'border-input hover:border-ring/40',
    )}
    aria-haspopup="listbox"
    aria-expanded={open}
    onclick={() => (open ? close() : openMenu())}
    onkeydown={onKeydown}
  >
    <span class={cn('truncate', selected ? 'text-foreground' : 'text-muted-foreground')}>
      {selected?.label ?? placeholder}
    </span>
  </button>

  {#if open}
    <div
      bind:this={panel}
      role="listbox"
      tabindex="-1"
      class="absolute left-0 top-full z-40 mt-2 w-full rounded-xl border border-border bg-popover p-1 shadow-lg"
    >
      <!-- 指向上方触发区的小三角 -->
      <span
        class="absolute -top-[5px] left-5 h-2.5 w-2.5 rotate-45 rounded-[1px] border-l border-t border-border bg-popover"
      ></span>
      {#each options as option, index (option.value)}
        <button
          type="button"
          role="option"
          aria-selected={option.value === value}
          disabled={option.disabled}
          class={cn(
            'block w-full rounded-md px-3 py-2 text-left text-sm transition-colors',
            index === activeIndex ? 'bg-muted text-foreground' : 'text-foreground hover:bg-muted',
            option.disabled && 'cursor-not-allowed opacity-50',
          )}
          onmouseenter={() => (activeIndex = index)}
          onclick={() => pick(option)}
        >
          {option.label}
        </button>
      {/each}
    </div>
  {/if}
</div>
