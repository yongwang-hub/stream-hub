<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '../utils';
  import Icon from './Icon.svelte';

  interface Props {
    variant?: 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive';
    size?: 'sm' | 'default' | 'lg' | 'icon';
    icon?: string;
    disabled?: boolean;
    type?: 'button' | 'submit';
    title?: string;
    class?: string;
    onclick?: () => void;
    children?: Snippet;
  }

  let {
    variant = 'default',
    size = 'default',
    icon,
    disabled = false,
    type = 'button',
    title,
    class: className = '',
    onclick,
    children,
  }: Props = $props();

  const variants: Record<string, string> = {
    default: 'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90',
    secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/70',
    outline: 'border border-input bg-background hover:bg-muted',
    ghost: 'hover:bg-muted text-foreground',
    destructive: 'bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90',
  };
  const sizes: Record<string, string> = {
    sm: 'h-9 px-3.5 text-xs gap-1.5',
    default: 'h-10 px-4 text-sm gap-2',
    lg: 'h-11 px-5 text-sm gap-2',
    icon: 'h-10 w-10 p-0',
  };
</script>

<button
  {type}
  {disabled}
  {title}
  {onclick}
  class={cn(
    'inline-flex shrink-0 items-center justify-center rounded-lg font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50',
    variants[variant],
    sizes[size],
    className,
  )}
>
  {#if icon}<Icon name={icon} size={size === 'sm' ? 15 : 17} />{/if}
  {@render children?.()}
</button>
