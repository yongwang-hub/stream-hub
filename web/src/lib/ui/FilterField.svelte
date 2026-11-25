<script lang="ts">
  // 筛选区单个字段：左侧文字标签 + 右侧控件（照公司 Vue XcSearchBar 的 label + control）。
  // 用 label 包住控件，点标签即可聚焦，不需要额外维护 id / for。
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';

  interface Props {
    label: string;
    class?: string;
    children?: Snippet;
  }

  let { label, class: className = '', children }: Props = $props();
</script>

<label class="flex items-center gap-2.5">
  <span class="shrink-0 whitespace-nowrap text-sm text-muted-foreground">{label}</span>
  <span class={cn('flex w-[220px] shrink-0', className)}>{@render children?.()}</span>
</label>
