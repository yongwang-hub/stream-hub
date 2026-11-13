<script lang="ts">
  // 统一图标组件：数据来自公司 xc-monorepo 的 iconfont（见 lib/icons.ts）。
  // 用 currentColor 继承文字颜色，明暗主题自动适配；size 走 width/height，可用 class 覆盖。
  import { icons } from '../icons';

  interface Props {
    name: string;
    size?: number;
    class?: string;
  }
  let { name, size = 16, class: className = '' }: Props = $props();
  const data = $derived(icons[name]);
</script>

{#if data}
  <svg
    width={size}
    height={size}
    viewBox={data.viewBox}
    fill="currentColor"
    class={className}
    aria-hidden="true"
    focusable="false"
  >
    {#each data.paths as d}
      <path {d} />
    {/each}
  </svg>
{:else}
  <span class="inline-block" style="width: {size}px; height: {size}px" aria-hidden="true"></span>
{/if}
