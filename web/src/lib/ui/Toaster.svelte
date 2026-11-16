<script lang="ts">
  // 全局消息弹窗：数值对齐公司 Vue 项目实际使用的 Element Plus 2.11.3 ElMessage：
  //   el-message.css:          padding 11px 15px / gap 8px / font-size 14px / line-height 1
  //   shared-styles/themes:     --el-border-radius-base: 8px（覆盖 EP 默认 4px）
  //   el-message--error:        bg=#fef0f0 border=#fbc4c4 text=#f56c6c
  import { toastItems, type ToastType } from './toast.svelte';

  const palette: Record<ToastType, { color: string; bg: string; border: string }> = {
    success: { color: '#67c23a', bg: '#f0f9eb', border: '#e1f3d8' },
    error: { color: '#f56c6c', bg: '#fef0f0', border: '#fbc4c4' },
    warning: { color: '#e6a23c', bg: '#fdf6ec', border: '#f5dab1' },
    info: { color: '#909399', bg: '#f4f4f5', border: '#e9e9eb' },
  };
</script>

<div class="pointer-events-none fixed left-1/2 top-4 z-[100] flex -translate-x-1/2 flex-col items-center gap-2">
  {#each toastItems() as t (t.id)}
    <div
      class="ui-fade-in inline-flex items-center gap-2 rounded-lg border px-[15px] py-[11px] text-[14px] font-normal leading-none shadow-sm"
      style="background:{palette[t.type].bg};border-color:{palette[t.type].border};color:{palette[t.type].color}"
      role="alert"
    >
      <svg class="h-4 w-4 shrink-0" viewBox="0 0 16 16" fill="none">
        {#if t.type === 'success'}
          <circle cx="8" cy="8" r="7" fill="currentColor" />
          <path d="M4.6 8.1 6.9 10.4 11.4 5.9" stroke="#fff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        {:else if t.type === 'error'}
          <circle cx="8" cy="8" r="7" fill="currentColor" />
          <path d="M5.5 5.5 10.5 10.5M10.5 5.5 5.5 10.5" stroke="#fff" stroke-width="1.5" stroke-linecap="round" />
        {:else if t.type === 'warning'}
          <path d="M8 1.5 15 14H1L8 1.5Z" fill="currentColor" />
          <path d="M8 5.8v3.6" stroke="#fff" stroke-width="1.5" stroke-linecap="round" />
          <circle cx="8" cy="11.6" r="0.85" fill="#fff" />
        {:else}
          <circle cx="8" cy="8" r="7" fill="currentColor" />
          <path d="M8 7v4.2" stroke="#fff" stroke-width="1.5" stroke-linecap="round" />
          <circle cx="8" cy="4.8" r="0.85" fill="#fff" />
        {/if}
      </svg>
      <span class="whitespace-nowrap">{t.message}</span>
    </div>
  {/each}
</div>
