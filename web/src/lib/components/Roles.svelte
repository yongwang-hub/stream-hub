<script lang="ts">
  // 角色管理：真接口 /api/v1/roles（sys_roles 表）。
  // 内置 admin 角色由 seed 创建，后端禁止改名/删除，前端也做了禁用提示。
  import { onMount } from 'svelte';
  import { ApiError, api } from '../api';
  import type { AdminRole, AdminRoleInput } from '../api';

  let roles = $state<AdminRole[]>([]);
  let loading = $state(false);
  let error = $state('');
  let notice = $state('');
  let keyword = $state('');
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
    err instanceof ApiError ? err.message : err instanceof Error ? err.message : String(err);

  async function load(): Promise<void> {
    loading = true;
    error = '';
    try {
      roles = await api.listRoles();
    } catch (err) {
      error = describe(err);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });

  function openCreate(): void {
    editing = null;
    form = { name: '', description: '' };
    error = '';
    notice = '';
    showModal = true;
  }

  function openEdit(role: AdminRole): void {
    editing = role;
    form = { name: role.name, description: role.description };
    error = '';
    notice = '';
    showModal = true;
  }

  async function save(): Promise<void> {
    if (!form.name.trim()) {
      error = '请填写角色名称';
      return;
    }
    const payload: AdminRoleInput = {
      name: form.name.trim(),
      description: form.description,
    };
    // 新建角色默认无权限；编辑时不传 permissions，后端保持原有权限不变。
    if (!editing) payload.permissions = [];
    error = '';
    try {
      if (editing) await api.updateRole(editing.id, payload);
      else await api.createRole(payload);
      showModal = false;
      notice = '已保存到数据库';
      await load();
    } catch (err) {
      error = describe(err);
    }
  }

  async function remove(role: AdminRole): Promise<void> {
    if (!confirm(`确定删除角色「${role.name}」吗？`)) return;
    error = '';
    try {
      await api.deleteRole(role.id);
      notice = '已删除';
      await load();
    } catch (err) {
      error = describe(err);
    }
  }
</script>

<h2 class="page-title">角色管理</h2>
<p class="page-hint">
  已对接真实 RBAC 接口 <code>/api/v1/roles</code>（sys_roles 表）。内置 admin 角色受保护，不能改名或删除。
  权限码在列表里展示；后端已支持 <code>permissions</code> 字段分配权限，勾选 UI 后续补。
</p>

{#if error}<div class="alert" style="margin-bottom: 12px">{error}</div>{/if}
{#if notice}<div class="note" style="margin-bottom: 12px">{notice}</div>{/if}

<div class="panel">
  <div class="toolbar">
    <input class="search" placeholder="请输入角色ID / 角色名称" bind:value={keyword} />
    <button class="btn primary sm" onclick={openCreate}>+ 新增</button>
    <span class="spacer"></span>
    <button class="btn sm" onclick={() => void load()} disabled={loading}>
      {loading ? '加载中…' : '刷新'}
    </button>
    <button class="btn ghost sm" onclick={() => (keyword = '')}>重置</button>
  </div>

  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>角色ID</th>
          <th>角色名称</th>
          <th>描述</th>
          <th>权限</th>
          <th style="text-align: right">操作</th>
        </tr>
      </thead>
      <tbody>
        {#each filtered as role (role.id)}
          <tr>
            <td class="mono">{role.id}</td>
            <td>
              {role.name}
              {#if role.name === 'admin'}<span class="tag" style="margin-left: 6px">内置</span>{/if}
            </td>
            <td>{role.description || '—'}</td>
            <td>
              {#each role.permissions.slice(0, 2) as code (code)}
                <span class="tag" style="margin-right: 4px">{code}</span>
              {/each}
              <span style="font-size: 11px; color: var(--muted)">
                {role.permissions.length ? `共 ${role.permissions.length} 项` : '无权限'}
              </span>
            </td>
            <td class="actions">
              <button class="icon-btn" onclick={() => openEdit(role)}>编辑</button>
              <button
                class="icon-btn del"
                disabled={role.name === 'admin'}
                title={role.name === 'admin' ? '内置角色不能删除' : '删除角色'}
                onclick={() => void remove(role)}
              >删除</button>
            </td>
          </tr>
        {:else}
          <tr><td colspan="5" class="empty">{loading ? '加载中…' : '暂无数据'}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>

  <div class="pager">
    <span>共 {filtered.length} 条</span>
    <span>10 条/页</span>
    <span>第 1 页</span>
  </div>
</div>

{#if showModal}
  <div class="modal-mask">
    <div class="modal">
      <h3>{editing ? '编辑角色' : '新增角色'}</h3>
      <div class="form-grid">
        <div class="field">
          <label for="r-name">角色名称</label>
          <input
            id="r-name"
            bind:value={form.name}
            disabled={editing?.name === 'admin'}
            placeholder="请输入角色名称"
          />
        </div>
        <div class="field">
          <label for="r-desc">描述</label>
          <input id="r-desc" bind:value={form.description} placeholder="选填" />
        </div>
      </div>
      <div class="modal-actions">
        <button class="btn ghost" onclick={() => (showModal = false)}>取消</button>
        <button class="btn primary" onclick={() => void save()}>保存</button>
      </div>
    </div>
  </div>
{/if}
