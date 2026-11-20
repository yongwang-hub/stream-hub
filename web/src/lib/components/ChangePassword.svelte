<script lang="ts">
  import { api } from '../api';
  import Icon from '../ui/Icon.svelte';
  import { toast } from '../ui/toast.svelte';

  interface Props {
    // force = 登录时强制改密（不可跳过）；active = 系统内主动改密（可取消）
    mode?: 'force' | 'active';
    onDone: (token: string) => void;
    onCancel?: () => void;
  }
  let { mode = 'force', onDone, onCancel }: Props = $props();

  const isForce = $derived(mode === 'force');

  // 弹窗结构对齐公司 Vue 源码：
  //   强制改密 widget/force-change-password/index.vue：500px 居中、不可关闭、warning alert、只有主按钮；
  //   主动改密 xc-user-popover/widget/modify-password/index.vue：可取消、无 warning、按钮为取消/提交。
  const fieldInput =
    'h-9 w-full rounded-[4px] border border-[#dcdfe6] bg-white px-3 text-[14px] text-[#606266] outline-none transition-colors placeholder:text-[#c0c4cc] focus:border-[#006be6]';

  let oldPassword = $state('');
  let newPassword = $state('');
  let confirm = $state('');
  let busy = $state(false);
  let newInput = $state<HTMLInputElement>();
  let confirmInput = $state<HTMLInputElement>();

  const describe = (err: unknown): string =>
    err instanceof Error ? err.message : String(err);

  // 回车：原密码/新密码跳到下一行，确认密码才提交。
  function handleEnter(event: KeyboardEvent, next?: HTMLInputElement): void {
    if (event.key !== 'Enter' || busy) return;
    event.preventDefault();
    if (next) next.focus();
    else void doSubmit();
  }

  function submit(event: SubmitEvent): void {
    event.preventDefault();
    void doSubmit();
  }

  async function doSubmit(): Promise<void> {
    if (busy) return;
    if (!oldPassword) {
      toast.error(isForce ? '请输入原密码' : '原始密码不能为空');
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
      toast.success(isForce ? '密码修改成功' : '修改密码成功');
      onDone(result.token);
    } catch (err) {
      toast.error(describe(err));
    } finally {
      busy = false;
    }
  }
</script>

<!-- 全屏遮罩：对齐 el-dialog 的 modal；强制改密不可点遮罩/按 Esc 关闭、无 X 关闭按钮 -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
  <div
    class="w-full max-w-[500px] overflow-hidden rounded-[4px] bg-white text-[#303133] shadow-2xl ui-fade-in"
    role="dialog"
    aria-modal="true"
    aria-label={isForce ? '密码需要修改' : '修改密码'}
  >
    <div class="px-5 pt-5 pb-3">
      <h2 class="text-[18px] font-medium leading-6 text-[#303133]">
        {isForce ? '密码需要修改' : '修改密码'}
      </h2>
    </div>

    <form onsubmit={submit}>
      <div class="px-5 pb-2">
        {#if isForce}
          <div class="mb-4 flex items-start gap-2 rounded-[4px] border border-[#faecd8] bg-[#fdf6ec] px-3 py-2 text-[13px] leading-5 text-[#e6a23c]">
            <Icon name="warning" size={16} class="mt-0.5 shrink-0" />
            <span>您的密码已过期或从未修改过，为保障账号安全，必须先修改密码才能进入系统。</span>
          </div>
        {/if}

        <div class="space-y-4">
          <div class="flex items-center gap-3">
            <label class="w-[100px] shrink-0 text-right text-[14px] text-[#606266]" for="old">
              {isForce ? '原密码' : '原始密码'}
            </label>
            <input id="old" class={fieldInput} type="password" bind:value={oldPassword} onkeydown={(e) => handleEnter(e, newInput)} autocomplete="current-password" placeholder="请输入原密码" />
          </div>
          <div class="flex items-center gap-3">
            <label class="w-[100px] shrink-0 text-right text-[14px] text-[#606266]" for="new">新密码</label>
            <input id="new" class={fieldInput} type="password" bind:value={newPassword} bind:this={newInput} onkeydown={(e) => handleEnter(e, confirmInput)} autocomplete="new-password" placeholder="请输入新密码" />
          </div>
          <div class="flex items-center gap-3">
            <label class="w-[100px] shrink-0 text-right text-[14px] text-[#606266]" for="confirm">确认密码</label>
            <input id="confirm" class={fieldInput} type="password" bind:value={confirm} bind:this={confirmInput} onkeydown={(e) => handleEnter(e)} autocomplete="new-password" placeholder="请再次输入新密码" />
          </div>
        </div>
      </div>

      <div class="mt-4 flex justify-end gap-3 border-t border-[#ebeef5] px-5 py-4">
        {#if !isForce && onCancel}
          <button type="button" onclick={onCancel} class="h-9 rounded-[4px] border border-[#dcdfe6] bg-white px-4 text-[14px] text-[#606266] transition-colors hover:border-[#006be6] hover:text-[#006be6]">
            取消
          </button>
        {/if}
        <button type="submit" disabled={busy} class="h-9 rounded-[4px] bg-[#006be6] px-4 text-[14px] text-white transition-opacity hover:opacity-90 disabled:opacity-60">
          {busy ? '提交中…' : '提交'}
        </button>
      </div>
    </form>
  </div>
</div>
