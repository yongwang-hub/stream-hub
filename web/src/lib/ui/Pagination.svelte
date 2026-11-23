<script lang="ts">
  import Button from './Button.svelte';

  interface Props {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
    disabled?: boolean;
    ongoto: (page: number) => void;
  }

  let {
    page,
    pageSize,
    total,
    totalPages,
    disabled = false,
    ongoto,
  }: Props = $props();
</script>

<span>共 {total} 条</span>
<span>{pageSize} 条/页</span>
<span>第 {page} / {totalPages} 页</span>
<Button variant="outline" size="sm" disabled={page <= 1 || disabled} onclick={() => ongoto(page - 1)}>上一页</Button>
<Button variant="outline" size="sm" disabled={page >= totalPages || disabled} onclick={() => ongoto(page + 1)}>下一页</Button>
