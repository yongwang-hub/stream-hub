<script lang="ts">
  // 列表页标准布局：上半卡片放筛选（自然高度，操作按钮靠右），下半卡片放工具栏（靠左）+ 表格 + 分页（居中）。
  // 参考 xc-monorepo 的 XcFullScreen + XcSearchBar + XcTableActionBar + XcTable 组合，
  // 这里把它收敛成单个组件，五个片段各司其职。卡片间距与监控页网格的 gap-6 保持一致。
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';

  interface Props {
    class?: string;
    filters?: Snippet;
    filterActions?: Snippet;
    toolbar?: Snippet;
    pagination?: Snippet;
    children: Snippet;
  }

  let {
    class: className = '',
    filters,
    filterActions,
    toolbar,
    pagination,
    children,
  }: Props = $props();
</script>

<div class={cn('flex min-h-full flex-1 flex-col gap-6', className)}>
  {#if filters}
    <div class="shrink-0 rounded-xl border border-border bg-card px-5 py-4 shadow-sm">
      <div class="flex flex-wrap items-center gap-x-8 gap-y-3">
        {@render filters()}
        <!-- 重置/查询等操作放最右侧（照公司 Vue 的 XcSearchBar） -->
        {#if filterActions}
          <div class="ml-auto flex shrink-0 items-center gap-2">{@render filterActions()}</div>
        {/if}
      </div>
    </div>
  {/if}

  <div class="flex min-h-0 flex-1 flex-col rounded-xl border border-border bg-card p-4 shadow-sm">
    {#if toolbar}
      <div class="mb-4 flex shrink-0 flex-wrap items-center justify-start gap-2">
        {@render toolbar()}
      </div>
    {/if}
    <div class="ui-scroll min-h-0 flex-1 overflow-x-auto">
      {@render children()}
    </div>
    {#if pagination}
      <!-- 分页区上方加一条分隔线，和表格区拉开层次（照公司 Vue 的 XcTable 分页样式） -->
      <div
        class="mt-4 flex shrink-0 flex-wrap items-center justify-center gap-3 border-t border-border pt-4 text-xs text-muted-foreground"
      >
        {@render pagination()}
      </div>
    {/if}
  </div>
</div>
