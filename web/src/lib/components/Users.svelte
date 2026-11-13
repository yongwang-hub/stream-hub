<script lang="ts">
  // 用户管理：真接口 /api/v1/users（sys_users 表）。
  // 列表/新增/编辑/删除/启停/分配角色都走后端；角色下拉来自 /api/v1/roles。
  import { onMount } from 'svelte';
  import { ApiError, api } from '../api';
  import type { AdminRole, AdminUser, AdminUserInput } from '../api';

  let users = $state<AdminUser[]>([]);
  let roleOptions = $state<AdminRole[]>([]);
  let loading = $state(false);
  let error = $state('');
  let notice = $state('');
  let keyword = $state('');
  let showModal = $state(false);
  let editing = $state<AdminUser | null>(null);
  let form = $state({
    username: '',
    password: '',
    email: '',
    enabled: true,
    role_ids: [] as number[],
  });

  const filtered = $derived(
    users.filter(
      (u) =>
        u.username.toLowerCase().includes(keyword.toLowerCase()) ||
        u.email.toLowerCase().includes(keyword.toLowerCase()) ||
        String(u.id).includes(keyword),
    ),
  );

  const describe = (err: unknown): string =>
    err instanceof ApiError ? err.message : err instanceof Error ? err.message : String(err);

  async function load(): Promise<void> {
    loading = true;
    error = '';
    try {
      const [u, r] = await Promise.all([api.listAdminUsers(), api.listRoles()]);
      users = u;
      roleOptions = r;
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
    form = { username: '', password: '', email: '', enabled: true, role_ids: [] };
    error = '';
    notice = '';
    showModal = true;
  }

  function openEdit(user: AdminUser): void {
    editing = user;
    form = {
      username: user.username,
      password: '',
      email: user.email,
      enabled: user.enabled,
      role_ids: [...user.role_ids],
    };
    error = '';
    notice = '';
    showModal = true;
  }

  function toggleRole(id: number): void {
    form.role_ids = form.role_ids.includes(id)
      ? form.role_ids.filter((x) => x !== id)
      : [...form.role_ids, id];
  }

  async function save(): Promise<void> {
    if (!form.username.trim()) {
      error = '请填写用户名';
      return;
    }
    if (!editing && !form.password) {
      error = '新增用户必须设置初始密码';
      return;
    }
    const payload: AdminUserInput = {
      username: form.username.trim(),
      email: form.email,
      enabled: form.enabled,
      role_ids: form.role_ids,
    };
    if (form.password) payload.password = form.password;
    error = '';
    try {
      if (editing) await api.updateAdminUser(editing.id, payload);
      else await api.createAdminUser(payload);
      showModal = false;
      notice = '已保存到数据库';
      await load();
    } catch (err) {
      error = describe(err);
    }
  }

  async function toggleEnabled(user: AdminUser): Promise<void> {
    error = '';
    try {
      // 只传 enabled：后端其余字段留空即不更新，角色关联也不会被动到。
      await api.updateAdminUser(user.id, { username: user.username, enabled: !user.enabled });
      notice = user.enabled ? '已停用' : '已启用';
      await load();
    } catch (err) {
      error = describe(err);
    }
  }

  async function remove(user: AdminUser): Promise<void> {
    if (!confirm(`确定删除用户「${user.username}」吗？`)) return;
    error = '';
    try {
      await api.deleteAdminUser(user.id);
      notice = '已删除';
      await load();
    } catch (err) {
      error = describe(err);
    }
  }
</script>

<h2 class="page-title">用户管理</h2>
<p class="page-hint">
  已对接真实账号接口 <code>/api/v1/users</code>（sys_users 表）。可新增用户、设置初始密码、
  分配角色、启停账号；新建用户首登会被强制改密。内置 admin 账号不能删除自己。
</p>

{#if error}<div class="alert" style="margin-bottom: 12px">{error}</div>{/if}
{#if notice}<div class="note" style="margin-bottom: 12px">{notice}</div>{/if}

<div class="panel">
  <div class="toolbar">
    <input class="search" placeholder="请输入用户名 / 邮箱" bind:value={keyword} />
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
          <th>ID</th>
          <th>用户名</th>
          <th>邮箱</th>
          <th>用户角色</th>
          <th>状态</th>
          <th style="text-align: right">操作</th>
        </tr>
      </thead>
      <tbody>
        {#each filtered as user (user.id)}
          <tr>
            <td class="mono">{user.id}</td>
            <td><span class="avatar">{user.username.slice(0, 1).toUpperCase()}</span> {user.username}</td>
            <td>{user.email || '—'}</td>
            <td>{user.roles.length ? user.roles.join('、') : '—'}</td>
            <td>
              <button
                class="switch {user.enabled ? '' : 'off'}"
                onclick={() => void toggleEnabled(user)}
                title="点击切换启停状态"
              ><i></i>{user.enabled ? '启用' : '停用'}</button>
            </td>
            <td class="actions">
              <button class="icon-btn" onclick={() => openEdit(user)}>编辑</button>
              <button class="icon-btn del" onclick={() => void remove(user)}>删除</button>
            </td>
          </tr>
        {:else}
          <tr><td colspan="6" class="empty">{loading ? '加载中…' : '暂无数据'}</td></tr>
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
      <h3>{editing ? '编辑用户' : '新增用户'}</h3>
      <div class="form-grid">
        <div class="field">
          <label for="u-name">用户名</label>
          <input id="u-name" bind:value={form.username} disabled={editing?.username === 'admin'} placeholder="请输入用户名" />
        </div>
        <div class="field">
          <label for="u-pwd">{editing ? '密码（留空不改）' : '初始密码'}</label>
          <input id="u-pwd" type="password" bind:value={form.password} placeholder={editing ? '留空表示不修改' : '必填'} />
        </div>
        <div class="field">
          <label for="u-email">邮箱</label>
          <input id="u-email" bind:value={form.email} placeholder="请输入邮箱" />
        </div>
        <div class="field">
          <label for="u-enabled">账号状态</label>
          <label class="inline" style="padding-top: 9px">
            <input id="u-enabled" type="checkbox" bind:checked={form.enabled} style="width: auto" />
            启用
          </label>
        </div>
        <div class="field" style="grid-column: 1 / -1">
          <span style="font-size: 12px; color: var(--muted)">分配角色</span>
          <div class="role-checks">
            {#each roleOptions as role (role.id)}
              <label>
                <input
                  type="checkbox"
                  checked={form.role_ids.includes(role.id)}
                  onchange={() => toggleRole(role.id)}
                />
                {role.name}
              </label>
            {:else}
              <span style="font-size: 12px; color: var(--muted)">暂无可选角色，请先到角色管理创建</span>
            {/each}
          </div>
        </div>
      </div>
      <div class="modal-actions">
        <button class="btn ghost" onclick={() => (showModal = false)}>取消</button>
        <button class="btn primary" onclick={() => void save()}>保存</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .role-checks { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 6px; }
  .role-checks label {
    display: inline-flex; align-items: center; gap: 6px; padding: 6px 10px;
    border: 1px solid var(--border); border-radius: 8px; font-size: 12px; cursor: pointer;
  }
  .role-checks input { width: auto; }
  .switch { background: transparent; border: none; padding: 0; cursor: pointer; }
  .switch.off i { background: var(--muted); }
  .switch.off i::after { right: auto; left: 2px; }
</style>
