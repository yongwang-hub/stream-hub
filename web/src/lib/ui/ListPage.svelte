<script lang="ts">
  // 列表页标准布局：上半卡片放筛选（自然高度），下半卡片放工具栏 + 表格 + 分页（铺满剩余高度）。
  // 参考 xc-monorepo 的 XcFullScreen + XcSearchBar + XcTableActionBar + XcTable 组合，
  // 这里把它收敛成单个组件，四个片段各司其职。
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';

  interface Props {
    class?: string;
    filters?: Snippet;
    toolbar?: Snippet;
    pagination?: Snippet;
    children: Snippet;
  }

  let {
    class: className = '',
    filters,
    toolbar,
    pagination,
    children,
  }: Props = $props();
</script>

<div class={cn('flex min-h-full flex-1 flex-col gap-3', className)}>
  {#if filters}
    <div class="shrink-0 rounded-xl border border-border bg-card px-4 py-3 shadow-sm">
      <div class="flex flex-wrap items-center gap-2">
        {@render filters()}
      </div>
    </div>
  {/if}

  <div class="flex min-h-0 flex-1 flex-col rounded-xl border border-border bg-card p-4 shadow-sm">
    {#if toolbar}
      <div class="mb-4 flex shrink-0 flex-wrap items-center justify-end gap-2">
        {@render toolbar()}
      </div>
    {/if}
    <div class="ui-scroll min-h-0 flex-1 overflow-x-auto">
      {@render children()}
    </div>
    {#if pagination}
      <div class="mt-4 flex shrink-0 flex-wrap items-center justify-end gap-3 text-xs text-muted-foreground">
        {@render pagination()}
      </div>
    {/if}
  </div>
</div>
