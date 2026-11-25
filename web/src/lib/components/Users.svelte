<script lang="ts">
  // 用户管理：真接口 /api/v1/users（sys_users 表）。
  // 列表/新增/编辑/删除/启停/分配角色全部走后端，角色选项来自 /api/v1/roles。
  // 报错/成功提示统一走全局 Toast，页面内不再放内嵌提示条。
  import { onMount } from 'svelte';
  import { api } from '../api';
  import type { AdminRole, AdminUser, AdminUserInput } from '../api';
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

  let users = $state<AdminUser[]>([]);
  let roleOptions = $state<AdminRole[]>([]);
  let loading = $state(false);
  let keyword = $state('');
  let page = $state(1);
  let total = $state(0);
  const pageSize = 20;
  const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
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
    err instanceof Error ? err.message : String(err);

  async function load(): Promise<void> {
    loading = true;
    try {
      const [u, r] = await Promise.all([
        api.listAdminUsers(page, pageSize),
        api.listRoles(1, 100),
      ]);
      users = u.items ?? [];
      total = u.total;
      roleOptions = r.items ?? [];
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
    form = { username: '', password: '', email: '', enabled: true, role_ids: [] };
    showModal = true;
  }

  function openEdit(user: AdminUser): void {
    editing = user;
    form = {
      username: user.username,
      password: '',
      email: user.email,
      enabled: user.enabled,
      role_ids: [...(user.role_ids ?? [])],
    };
    showModal = true;
  }

  function toggleRole(id: number): void {
    form.role_ids = form.role_ids.includes(id)
      ? form.role_ids.filter((x) => x !== id)
      : [...form.role_ids, id];
  }

  async function save(): Promise<void> {
    if (!form.username.trim()) {
      toast.error('请填写用户名');
      return;
    }
    if (!editing && !form.password) {
      toast.error('新增用户必须设置初始密码');
      return;
    }
    const payload: AdminUserInput = {
      username: form.username.trim(),
      email: form.email,
      enabled: form.enabled,
      role_ids: form.role_ids,
    };
    if (form.password) payload.password = form.password;
    try {
      if (editing) await api.updateAdminUser(editing.id, payload);
      else await api.createAdminUser(payload);
      showModal = false;
      toast.success('已保存到数据库');
      await load();
    } catch (err) {
      toast.error(describe(err));
    }
  }

  async function toggleEnabled(user: AdminUser): Promise<void> {
    try {
      await api.updateAdminUser(user.id, { username: user.username, enabled: !user.enabled });
      toast.success(user.enabled ? '已停用' : '已启用');
      await load();
    } catch (err) {
      toast.error(describe(err));
    }
  }

  async function remove(user: AdminUser): Promise<void> {
    if (!confirm(`确定删除用户「${user.username}」吗？`)) return;
    try {
      await api.deleteAdminUser(user.id);
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
      <input class={inputClass} placeholder="用户名 / 邮箱" bind:value={keyword} />
    </FilterField>
  {/snippet}

  {#snippet filterActions()}
    <Button variant="outline" onclick={() => (keyword = '')}>重置</Button>
    <Button onclick={() => void load()} disabled={loading}>刷新</Button>
  {/snippet}

  {#snippet toolbar()}
    <Button size="sm" icon="plus" onclick={openCreate}>新增用户</Button>
  {/snippet}

  <table class={tableClass}>
        <thead class="bg-muted/40">
          <tr>
            <th class={thClass}>ID</th>
            <th class={thClass}>用户名</th>
            <th class={thClass}>邮箱</th>
            <th class={thClass}>用户角色</th>
            <th class={thClass}>状态</th>
            <th class={cn(thClass, 'text-right')}>操作</th>
          </tr>
        </thead>
        <tbody>
          {#each filtered as user (user.id)}
            <tr class={rowHoverClass}>
              <td class={cn(tdClass, 'font-mono text-xs text-muted-foreground')}>{user.id}</td>
              <td class={tdClass}>
                <div class="flex items-center gap-2">
                  <span class="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                    {user.username.slice(0, 1).toUpperCase()}
                  </span>
                  <span class="font-medium">{user.username}</span>
                </div>
              </td>
              <td class={cn(tdClass, 'text-muted-foreground')}>{user.email || '—'}</td>
              <td class={tdClass}>
                <div class="flex flex-wrap gap-1">
                  {#each user.roles as roleName}
                    <Badge variant="secondary">{roleName}</Badge>
                  {:else}
                    <span class="text-muted-foreground">—</span>
                  {/each}
                </div>
              </td>
              <td class={tdClass}>
                <button onclick={() => void toggleEnabled(user)} title="点击切换启停状态">
                  <Badge variant={user.enabled ? 'success' : 'secondary'}>
                    {user.enabled ? '● 启用' : '○ 停用'}
                  </Badge>
                </button>
              </td>
              <td class={cn(tdClass, 'text-right whitespace-nowrap')}>
                <Button variant="ghost" size="sm" icon="edit" onclick={() => openEdit(user)}>编辑</Button>
                <Button
                  variant="ghost"
                  size="sm"
                  icon="trash"
                  class="text-destructive hover:text-destructive"
                  onclick={() => void remove(user)}
                >删除</Button>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="px-4 py-12 text-center text-sm text-muted-foreground">
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
  title={editing ? '编辑用户' : '新增用户'}
  description={editing ? '密码留空表示不修改' : '新建用户首次登录会被强制改密'}
  width="max-w-xl"
  onClose={() => (showModal = false)}
>
  <div class="grid gap-4 sm:grid-cols-2">
    <div>
      <label class={labelClass} for="u-name">用户名</label>
      <input id="u-name" class={inputClass} bind:value={form.username} disabled={editing?.username === 'admin'} placeholder="请输入用户名" />
    </div>
    <div>
      <label class={labelClass} for="u-pwd">{editing ? '密码（留空不改）' : '初始密码'}</label>
      <input id="u-pwd" class={inputClass} type="password" bind:value={form.password} placeholder={editing ? '留空表示不修改' : '必填'} />
    </div>
    <div>
      <label class={labelClass} for="u-email">邮箱</label>
      <input id="u-email" class={inputClass} bind:value={form.email} placeholder="选填" />
    </div>
    <div>
      <span class={labelClass}>账号状态</span>
      <label class="mt-2 flex cursor-pointer items-center gap-2 text-sm">
        <input type="checkbox" class="h-4 w-4 accent-primary" bind:checked={form.enabled} />
        启用
      </label>
    </div>
    <div class="sm:col-span-2">
      <span class={labelClass}>分配角色</span>
      <div class="flex flex-wrap gap-2">
        {#each roleOptions as role (role.id)}
          <label
            class={cn(
              'flex cursor-pointer items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs transition-colors',
              form.role_ids.includes(role.id)
                ? 'border-primary bg-primary/10 text-primary'
                : 'border-border text-muted-foreground hover:bg-muted',
            )}
          >
            <input
              type="checkbox"
              class="sr-only"
              checked={form.role_ids.includes(role.id)}
              onchange={() => toggleRole(role.id)}
            />
            {role.name}
          </label>
        {:else}
          <span class="text-xs text-muted-foreground">暂无可选角色，请先到角色管理创建</span>
        {/each}
      </div>
    </div>
  </div>

  {#snippet footer()}
    <Button variant="outline" onclick={() => (showModal = false)}>取消</Button>
    <Button onclick={() => void save()}>保存</Button>
  {/snippet}
</Modal>
