<script lang="ts">
  // 设备管理 / 摄像机管理：RTSP 设备只需名称、流地址与启停状态。
  import { onMount } from 'svelte';
  import { api, type Camera, type CameraInput } from '../api';
  import Badge from '../ui/Badge.svelte';
  import Button from '../ui/Button.svelte';
  import FilterField from '../ui/FilterField.svelte';
  import Modal from '../ui/Modal.svelte';
  import ListPage from '../ui/ListPage.svelte';
  import Pagination from '../ui/Pagination.svelte';
  import Select from '../ui/Select.svelte';
  import { inputClass, labelClass, tableClass, thClass, tdClass, rowHoverClass } from '../ui/classes';
  import { toast } from '../ui/toast.svelte';

  let rows = $state<Camera[]>([]);
  let total = $state(0);
  let loading = $state(false);
  let page = $state(1);
  const pageSize = 20;
  // 状态下拉固定三项：空串代表不过滤
  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'true', label: '启用' },
    { value: 'false', label: '停用' },
  ];
  const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));

  // 搜索框是草稿，点“查询”才生效，避免每敲一个字符都请求一次。
  let draft = $state({ keyword: '', status: '' });
  let applied = $state({ keyword: '', status: '' });

  let showModal = $state(false);
  let editing = $state<Camera | null>(null);
  let form = $state<CameraInput>(emptyForm());

  function emptyForm(): CameraInput {
    return { name: '', rtsp_url: '', enabled: true };
  }

  const describe = (err: unknown): string =>
    err instanceof Error ? err.message : String(err);

  async function load(): Promise<void> {
    loading = true;
    try {
      const result = await api.listCameras({
        page,
        pageSize,
        keyword: applied.keyword,
        enabled: applied.status === '' ? undefined : applied.status === 'true',
      });
      rows = result.items ?? [];
      total = result.total;
    } catch (err) {
      toast.error(describe(err));
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });

  async function search(): Promise<void> {
    applied = { ...draft };
    page = 1;
    await load();
  }

  async function reset(): Promise<void> {
    draft = { keyword: '', status: '' };
    applied = { ...draft };
    page = 1;
    await load();
  }

  async function gotoPage(next: number): Promise<void> {
    page = next;
    await load();
  }

  function openCreate(): void {
    editing = null;
    form = emptyForm();
    showModal = true;
  }

  function openEdit(row: Camera): void {
    editing = row;
    form = { name: row.name, rtsp_url: row.rtsp_url, enabled: row.enabled };
    showModal = true;
  }

  async function save(): Promise<void> {
    if (!form.name?.trim()) {
      toast.error('请填写设备名称');
      return;
    }
    const url = form.rtsp_url?.trim() ?? '';
    if (!/^rtsps?:\/\/.+/i.test(url)) {
      toast.error('RTSP 流地址需以 rtsp:// 或 rtsps:// 开头');
      return;
    }
    try {
      if (editing) await api.updateCamera(editing.id, { name: form.name, rtsp_url: url, enabled: form.enabled });
      else await api.createCamera({ name: form.name, rtsp_url: url, enabled: form.enabled });
      showModal = false;
      toast.success(editing ? '已保存' : '已新增摄像机');
      await load();
    } catch (err) {
      toast.error(describe(err));
    }
  }

  async function toggleEnabled(row: Camera): Promise<void> {
    try {
      await api.updateCamera(row.id, { enabled: !row.enabled });
      await load();
    } catch (err) {
      toast.error(describe(err));
    }
  }

  async function remove(row: Camera): Promise<void> {
    if (!confirm(`确定删除摄像机「${row.name}」吗？`)) return;
    try {
      await api.deleteCamera(row.id);
      toast.success('已删除');
      await load();
    } catch (err) {
      toast.error(describe(err));
    }
  }
</script>

<ListPage>
  {#snippet filters()}
    <FilterField label="关键词">
      <input class={inputClass} placeholder="设备名称 / RTSP 地址" bind:value={draft.keyword} />
    </FilterField>
    <FilterField label="状态">
      <Select bind:value={draft.status} options={statusOptions} />
    </FilterField>
  {/snippet}

  {#snippet filterActions()}
    <Button variant="outline" onclick={() => void reset()}>重置</Button>
    <Button onclick={() => void search()}>查询</Button>
  {/snippet}

  {#snippet toolbar()}
    <Button size="sm" icon="plus" onclick={openCreate}>新增摄像机</Button>
  {/snippet}

  <table class={tableClass}>
    <thead class="bg-muted/40">
      <tr>
        <th class={thClass}>设备名称</th>
        <th class={thClass}>RTSP 流地址</th>
        <th class={thClass}>状态</th>
        <th class={`${thClass} text-right`}>操作</th>
      </tr>
    </thead>
    <tbody>
      {#each rows as row (row.id)}
        <tr class={rowHoverClass}>
          <td class={`${tdClass} font-medium`}>{row.name}</td>
          <td class={`${tdClass} max-w-[480px] truncate font-mono text-xs`} title={row.rtsp_url}>{row.rtsp_url}</td>
          <td class={tdClass}>
            <button onclick={() => void toggleEnabled(row)} title="点击切换启停状态">
              <Badge variant={row.enabled ? 'success' : 'secondary'}>
                {row.enabled ? '● 启用' : '○ 停用'}
              </Badge>
            </button>
          </td>
          <td class={`${tdClass} text-right whitespace-nowrap`}>
            <Button variant="ghost" size="sm" icon="edit" onclick={() => openEdit(row)}>编辑</Button>
            <Button
              variant="ghost"
              size="sm"
              icon="trash"
              class="text-destructive hover:text-destructive"
              onclick={() => void remove(row)}
            >删除</Button>
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan="4" class="px-4 py-12 text-center text-sm text-muted-foreground">
            {loading ? '加载中…' : '暂无数据'}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  {#snippet pagination()}
    <Pagination {page} {pageSize} {total} {totalPages} disabled={loading} ongoto={(next) => void gotoPage(next)} />
  {/snippet}
</ListPage>

<Modal
  open={showModal}
  title={editing ? '编辑摄像机' : '新增摄像机'}
  description="仅支持 RTSP 接入；流地址中可包含用户名与密码"
  width="max-w-lg"
  onClose={() => (showModal = false)}
>
  <div class="grid gap-4">
    <div>
      <label class={labelClass} for="cam-name">设备名称</label>
      <input id="cam-name" class={inputClass} bind:value={form.name} placeholder="如：前门摄像头" />
    </div>
    <div>
      <label class={labelClass} for="cam-url">RTSP 流地址</label>
      <input id="cam-url" class={`${inputClass} font-mono text-xs`} bind:value={form.rtsp_url} placeholder="rtsp://192.168.1.64:554/live" />
    </div>
    <div>
      <label class="flex cursor-pointer items-center gap-2 text-sm">
        <input type="checkbox" class="h-4 w-4 accent-primary" bind:checked={form.enabled} />
        启用该摄像机
      </label>
    </div>
  </div>

  {#snippet footer()}
    <Button variant="outline" onclick={() => (showModal = false)}>取消</Button>
    <Button onclick={() => void save()}>保存</Button>
  {/snippet}
</Modal>
