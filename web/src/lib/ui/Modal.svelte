<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';
  import Icon from './Icon.svelte';

  interface Props {
    open: boolean;
    title: string;
    description?: string;
    width?: string;
    onClose: () => void;
    children?: Snippet;
    footer?: Snippet;
  }
  let {
    open,
    title,
    description,
    width = 'max-w-lg',
    onClose,
    children,
    footer,
  }: Props = $props();
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && open) onClose(); }} />

{#if open}
  <div
    class="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4 backdrop-blur-[2px]"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
  >
    <div
      class={cn(
        'w-full rounded-xl border border-border bg-card text-card-foreground shadow-2xl ui-fade-in',
        width,
      )}
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <header class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
        <div>
          <h3 class="text-lg font-semibold">{title}</h3>
          {#if description}<p class="mt-1 text-xs text-muted-foreground">{description}</p>{/if}
        </div>
        <button
          type="button"
          class="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          onclick={onClose}
          aria-label="关闭"
        >
          <Icon name="close" size={16} />
        </button>
      </header>
      <div class="px-5 py-5">{@render children?.()}</div>
      {#if footer}
        <footer class="flex justify-end gap-2 border-t border-border px-5 py-4">
          {@render footer()}
        </footer>
      {/if}
    </div>
  </div>
{/if}
