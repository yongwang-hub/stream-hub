<script lang="ts">
  // 角色管理：真接口 /api/v1/roles（sys_roles 表）。内置 admin 角色受后端保护。
  // 报错/成功提示统一走全局 Toast，页面内不再放内嵌提示条。
  import { onMount } from 'svelte';
  import { api } from '../api';
  import type { AdminRole, AdminRoleInput } from '../api';
  import FilterField from '../ui/FilterField.svelte';
  import Button from '../ui/Button.svelte';
  import Badge from '../ui/Badge.svelte';
  import Modal from '../ui/Modal.svelte';
  import ListPage from '../ui/ListPage.svelte';
  import Pagination from '../ui/Pagination.svelte';
  import { toast } from '../ui/toast.svelte';
  import { cn } from '../utils';
  import {
    inputClass,
    labelClass,
    tableClass,
    thClass,
    tdClass,
    rowHoverClass,
  } from '../ui/classes';

  let roles = $state<AdminRole[]>([]);
  let loading = $state(false);
  let keyword = $state('');
  let page = $state(1);
  let total = $state(0);
  const pageSize = 20;
  const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
  let showModal = $state(false);
  let editing = $state<AdminRole | null>(null);
  let form = $state({ name: '', description: '' });

  const filtered = $derived(
    roles.filter(
      (r) =>
        r.name.includes(keyword) ||
        r.description.includes(keyword) ||
        String(r.id).includes(keyword),
    ),
  );

  const describe = (err: unknown): string =>
    err instanceof Error ? err.message : String(err);

  async function load(): Promise<void> {
    loading = true;
    try {
      const result = await api.listRoles(page, pageSize);
      roles = result.items ?? [];
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

  async function gotoPage(next: number): Promise<void> {
    page = next;
    await load();
  }

  function openCreate(): void {
    editing = null;
    form = { name: '', description: '' };
    showModal = true;
  }

  function openEdit(role: AdminRole): void {
    editing = role;
    form = { name: role.name, description: role.description };
    showModal = true;
  }

  async function save(): Promise<void> {
    if (!form.name.trim()) {
      toast.error('请填写角色名称');
      return;
    }
    const payload: AdminRoleInput = {
      name: form.name.trim(),
      description: form.description,
    };
    if (!editing) payload.permissions = [];
    try {
      if (editing) await api.updateRole(editing.id, payload);
      else await api.createRole(payload);
      showModal = false;
      toast.success('已保存到数据库');
      await load();
    } catch (err) {
      toast.error(describe(err));
    }
  }

  async function remove(role: AdminRole): Promise<void> {
    if (!confirm(`确定删除角色「${role.name}」吗？`)) return;
    try {
      await api.deleteRole(role.id);
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
      <input class={inputClass} placeholder="角色名称 / ID" bind:value={keyword} />
    </FilterField>
  {/snippet}

  {#snippet filterActions()}
    <Button variant="outline" onclick={() => (keyword = '')}>重置</Button>
    <Button onclick={() => void load()} disabled={loading}>刷新</Button>
  {/snippet}

  {#snippet toolbar()}
    <Button size="sm" icon="plus" onclick={openCreate}>新增角色</Button>
  {/snippet}

  <table class={tableClass}>
        <thead class="bg-muted/40">
          <tr>
            <th class={thClass}>角色ID</th>
            <th class={thClass}>角色名称</th>
            <th class={thClass}>描述</th>
            <th class={thClass}>权限</th>
            <th class={cn(thClass, 'text-right')}>操作</th>
          </tr>
        </thead>
        <tbody>
          {#each filtered as role (role.id)}
            <tr class={rowHoverClass}>
              <td class={cn(tdClass, 'font-mono text-xs text-muted-foreground')}>{role.id}</td>
              <td class={tdClass}>
                <div class="flex items-center gap-2">
                  <span class="font-medium">{role.name}</span>
                  {#if role.name === 'admin'}<Badge variant="warning">内置</Badge>{/if}
                </div>
              </td>
              <td class={cn(tdClass, 'text-muted-foreground')}>{role.description || '—'}</td>
              <td class={tdClass}>
                <div class="flex flex-wrap items-center gap-1">
                  {#each (role.permissions ?? []).slice(0, 2) as code}
                    <Badge variant="outline">{code}</Badge>
                  {/each}
                  <span class="text-xs text-muted-foreground">
                    {(role.permissions ?? []).length ? `共 ${(role.permissions ?? []).length} 项` : '无权限'}
                  </span>
                </div>
              </td>
              <td class={cn(tdClass, 'text-right whitespace-nowrap')}>
                <Button variant="ghost" size="sm" icon="edit" onclick={() => openEdit(role)}>编辑</Button>
                <Button
                  variant="ghost"
                  size="sm"
                  icon="trash"
                  class="text-destructive hover:text-destructive"
                  disabled={role.name === 'admin'}
                  title={role.name === 'admin' ? '内置角色不能删除' : '删除角色'}
                  onclick={() => void remove(role)}
                >删除</Button>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="5" class="px-4 py-12 text-center text-sm text-muted-foreground">
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
  title={editing ? '编辑角色' : '新增角色'}
  description="编辑时不修改该角色已分配的权限"
  onClose={() => (showModal = false)}
>
  <div class="grid gap-4 sm:grid-cols-2">
    <div>
      <label class={labelClass} for="r-name">角色名称</label>
      <input id="r-name" class={inputClass} bind:value={form.name} disabled={editing?.name === 'admin'} placeholder="请输入角色名称" />
    </div>
    <div>
      <label class={labelClass} for="r-desc">描述</label>
      <input id="r-desc" class={inputClass} bind:value={form.description} placeholder="选填" />
    </div>
  </div>

  {#snippet footer()}
    <Button variant="outline" onclick={() => (showModal = false)}>取消</Button>
    <Button onclick={() => void save()}>保存</Button>
  {/snippet}
</Modal>
