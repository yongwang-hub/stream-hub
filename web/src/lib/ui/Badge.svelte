<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';

  interface Props {
    variant?: 'default' | 'secondary' | 'outline' | 'success' | 'warning' | 'destructive';
    class?: string;
    children?: Snippet;
  }
  let { variant = 'default', class: className = '', children }: Props = $props();

  const variants: Record<string, string> = {
    default: 'bg-primary/10 text-primary border-primary/20',
    secondary: 'bg-secondary text-secondary-foreground border-border',
    outline: 'bg-transparent text-muted-foreground border-border',
    success: 'bg-success/10 text-success border-success/20',
    warning: 'bg-warning/10 text-warning border-warning/20',
    destructive: 'bg-destructive/10 text-destructive border-destructive/20',
  };
</script>

<span
  class={cn(
    'inline-flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs font-medium whitespace-nowrap',
    variants[variant],
    className,
  )}
>
  {@render children?.()}
</span>
