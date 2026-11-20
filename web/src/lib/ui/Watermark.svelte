<script lang="ts">
  // 全局水印：对齐 Vue 系统配置的「全局水印」选项，斜向平铺「用户名 + 日期」。
  // pointer-events-none 保证不挡任何交互。
  import { config } from '../systemConfig.svelte';

  interface Props {
    username?: string;
  }
  let { username = 'admin' }: Props = $props();

  const now = new Date();
  const date = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
  const text = $derived(`${username}  ${date}`);
  const rows = Array.from({ length: 12 }, (_, i) => i);
</script>

{#if config().watermark}
  <div class="pointer-events-none fixed inset-0 z-[100] overflow-hidden" aria-hidden="true">
    <div class="absolute -inset-1/4 grid grid-cols-4 gap-x-16 gap-y-24 rotate-[-30deg] opacity-[0.07]">
      {#each rows as _}
        <span class="whitespace-nowrap text-[14px] font-medium text-foreground">{text}</span>
      {/each}
    </div>
  </div>
{/if}
