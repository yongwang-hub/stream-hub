<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';

  interface Props {
    title?: string;
    description?: string;
    class?: string;
    bodyClass?: string;
    children?: Snippet;
    actions?: Snippet;
  }
  let {
    title,
    description,
    class: className = '',
    bodyClass = '',
    children,
    actions,
  }: Props = $props();
</script>

<section
  class={cn(
    'rounded-xl border border-border bg-card text-card-foreground shadow-sm',
    className,
  )}
>
  {#if title || actions}
    <header class="flex flex-wrap items-start justify-between gap-3 border-b border-border px-5 py-4">
      <div>
        {#if title}<h3 class="text-base font-semibold text-foreground">{title}</h3>{/if}
        {#if description}<p class="mt-1 text-xs text-muted-foreground">{description}</p>{/if}
      </div>
      {#if actions}<div class="flex items-center gap-2">{@render actions()}</div>{/if}
    </header>
  {/if}
  <div class={cn('p-5', bodyClass)}>{@render children?.()}</div>
</section>
