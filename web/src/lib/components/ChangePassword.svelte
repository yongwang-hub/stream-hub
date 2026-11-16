<script lang="ts">
  import { api } from '../api';
  import Icon from '../ui/Icon.svelte';
  import { toast } from '../ui/toast.svelte';

  interface Props {
    onDone: (token: string) => void;
    onCancel: () => void;
  }
  let { onDone, onCancel }: Props = $props();

  // 按公司 Vue 源码 packages/shared-views/src/views/xc-login/widget/force-change-password/index.vue：
  // el-dialog 500px 居中、不可点遮罩/按 Esc 关闭、无 X；warning alert + label-width 100px 表单；
  // footer 为「返回登录」(白色次要按钮) + 「提交并进入系统」(primary 主按钮)。
  const fieldInput =
    'h-9 w-full rounded-[4px] border border-[#dcdfe6] bg-white px-3 text-[14px] text-[#606266] outline-none transition-colors placeholder:text-[#c0c4cc] focus:border-[#006be6]';

  let oldPassword = $state('');
  let newPassword = $state('');
  let confirm = $state('');
  let busy = $state(false);

  const describe = (err: unknown): string =>
    err instanceof Error ? err.message : String(err);

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!oldPassword) {
      toast.error('请输入原密码');
      return;
    }
    if (newPassword.length < 6) {
      toast.error('新密码至少 6 位');
      return;
    }
    if (newPassword !== confirm) {
      toast.error('两次输入的新密码不一致');
      return;
    }
    busy = true;
    try {
      const result = await api.changePassword(oldPassword, newPassword);
      toast.success('密码修改成功');
      onDone(result.token);
    } catch (err) {
      toast.error(describe(err));
    } finally {
      busy = false;
    }
  }
</script>

<!-- 全屏遮罩：对齐 el-dialog 的 modal；不可点遮罩/按 Esc 关闭，且没有 X 关闭按钮 -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
  <div
    class="w-full max-w-[500px] overflow-hidden rounded-[4px] bg-white text-[#303133] shadow-2xl ui-fade-in"
    role="dialog"
    aria-modal="true"
    aria-label="密码需要修改"
  >
    <div class="px-5 pt-5 pb-3">
      <h2 class="text-[18px] font-medium leading-6 text-[#303133]">密码需要修改</h2>
    </div>

    <form onsubmit={submit}>
      <div class="px-5 pb-2">
        <div class="mb-4 flex items-start gap-2 rounded-[4px] border border-[#faecd8] bg-[#fdf6ec] px-3 py-2 text-[13px] leading-5 text-[#e6a23c]">
          <Icon name="warning" size={16} class="mt-0.5 shrink-0" />
          <span>您的密码已过期或从未修改过，为保障账号安全，必须先修改密码才能进入系统。</span>
        </div>

        <div class="space-y-4">
          <div class="flex items-center gap-3">
            <label class="w-[100px] shrink-0 text-right text-[14px] text-[#606266]" for="old">原密码</label>
            <input id="old" class={fieldInput} type="password" bind:value={oldPassword} autocomplete="current-password" placeholder="请输入原密码" />
          </div>
          <div class="flex items-center gap-3">
            <label class="w-[100px] shrink-0 text-right text-[14px] text-[#606266]" for="new">新密码</label>
            <input id="new" class={fieldInput} type="password" bind:value={newPassword} autocomplete="new-password" placeholder="请输入新密码" />
          </div>
          <div class="flex items-center gap-3">
            <label class="w-[100px] shrink-0 text-right text-[14px] text-[#606266]" for="confirm">确认密码</label>
            <input id="confirm" class={fieldInput} type="password" bind:value={confirm} autocomplete="new-password" placeholder="请再次输入新密码" />
          </div>
        </div>
      </div>

      <div class="mt-4 flex justify-end gap-3 border-t border-[#ebeef5] px-5 py-4">
        <button type="button" onclick={onCancel} class="h-9 rounded-[4px] border border-[#dcdfe6] bg-white px-4 text-[14px] text-[#606266] transition-colors hover:border-[#006be6] hover:text-[#006be6]">
          返回登录
        </button>
        <button type="submit" disabled={busy} class="h-9 rounded-[4px] bg-[#006be6] px-4 text-[14px] text-white transition-opacity hover:opacity-90 disabled:opacity-60">
          {busy ? '提交中…' : '提交并进入系统'}
        </button>
      </div>
    </form>
  </div>
</div>
