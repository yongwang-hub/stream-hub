<script lang="ts">
  // 系统配置：按公司 Vue packages/shared-views/src/views/xc-setting/index.vue 的字段与样式。
  // 产品名称置顶；授权方式按需求去掉；型号/序列号/版本/授权/机器码/运行时长只读（后端启动时采集）。
  // 布局照 Vue 的 el-form：label 在左固定宽右对齐、控件在右，两列紧凑排布。
  import { api, uploadImage, type SystemConfigWritable } from '../api';
  import defaultLoginBg from '../../assets/login-bg.webp';
  import defaultLogo from '../../assets/logo.png';
  import { config, loadConfig, saveConfig } from '../systemConfig.svelte';
  import Button from '../ui/Button.svelte';
  import Icon from '../ui/Icon.svelte';
  import { inputClass } from '../ui/classes';
  import { toast } from '../ui/toast.svelte';

  // el-form label 风格：左列固定宽、右对齐、垂直居中。
  const fieldLabel = 'w-[100px] shrink-0 text-right text-[13px] text-muted-foreground leading-none';
  const row = 'flex items-center gap-3';

  // 进入页面先从后端拉最新配置（只读字段也在这里）。
  let loaded = $state(false);
  let busy = $state(false);
  let form = $state<SystemConfigWritable>({
    product_name: '',
    login_bg: '',
    product_icon: '',
    warning_text: '',
    watermark: false,
    activation_code: '',
  });

  $effect(() => {
    void api.getSystemConfig().then((cfg) => {
      form = {
        product_name: cfg.product_name,
        login_bg: cfg.login_bg,
        product_icon: cfg.product_icon,
        warning_text: cfg.warning_text,
        watermark: cfg.watermark,
        activation_code: cfg.activation_code,
      };
      void loadConfig();
      loaded = true;
    }).catch((err) => {
      loaded = true;
      toast.error(err instanceof Error ? err.message : '加载系统配置失败');
    });
  });

  // 运行时长用后端返回的 started_at 计算，每秒刷新显示。
  let ticks = $state(0);
  $effect(() => {
    const timer = setInterval(() => (ticks += 1), 1000);
    return () => clearInterval(timer);
  });
  const runTime = $derived.by(() => {
    void ticks;
    const raw = config().started_at;
    if (!raw) return '—';
    const start = Date.parse(raw);
    if (Number.isNaN(start)) return '—';
    return formatDuration(Math.max(0, Math.floor((Date.now() - start) / 1000)));
  });

  function formatDuration(total: number): string {
    const d = Math.floor(total / 86400);
    const h = Math.floor((total % 86400) / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    return `${d} 天 ${h} 小时 ${m} 分 ${s} 秒`;
  }

  function licenseText(days: number): string {
    return days >= 9999 ? '永久授权' : `剩余 ${days} 天`;
  }

  async function pickImage(field: 'login_bg' | 'product_icon', event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    try {
      const url = await uploadImage(file);
      form[field] = url;
      toast.success('图片已上传，记得点保存');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '图片上传失败');
    } finally {
      input.value = '';
    }
  }

  function removeImage(field: 'login_bg' | 'product_icon'): void {
    form[field] = '';
  }

  async function save(): Promise<void> {
    busy = true;
    try {
      await saveConfig({ ...form });
      toast.success('保存成功');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '保存失败');
    } finally {
      busy = false;
    }
  }
</script>

{#if !loaded}
  <p class="text-sm text-muted-foreground">正在加载系统配置…</p>
{:else}
<div class="flex min-h-full flex-1 flex-col">
  <div class="flex min-h-0 flex-1 flex-col rounded-xl border border-border bg-card text-card-foreground shadow-sm">
    <header class="flex shrink-0 flex-wrap items-start justify-between gap-3 border-b border-border px-5 py-4">
      <h3 class="text-base font-semibold text-foreground">系统配置</h3>
    </header>
    <div class="min-h-0 flex-1 p-5">
      <div class="grid gap-x-6 gap-y-4 sm:grid-cols-2">
        <!-- 产品名称 | 产品版本 -->
        <div class={row}>
          <label class={fieldLabel} for="c-name"><span class="text-destructive">*</span> 产品名称</label>
          <input id="c-name" class={inputClass} bind:value={form.product_name} />
        </div>

        <div class={row}>
          <label class={fieldLabel} for="c-version">产品版本</label>
          <input id="c-version" class={inputClass} value={config().product_version} disabled />
        </div>

        <!-- 产品型号 | 产品序列号 -->
        <div class={row}>
          <label class={fieldLabel} for="c-model">产品型号</label>
          <input id="c-model" class={inputClass} value={config().product_model} disabled />
        </div>

        <div class={row}>
          <label class={fieldLabel} for="c-serial">产品序列号</label>
          <input id="c-serial" class={inputClass} value={config().serial_number} disabled />
        </div>

        <!-- 登录背景 | 产品图标 -->
        <div class={row}>
          <label class={fieldLabel} for="c-bg">登录背景</label>
          <label
            for="c-bg"
            class="relative flex h-[64px] w-[130px] cursor-pointer items-center justify-center overflow-hidden rounded-md border border-input bg-muted/50 text-muted-foreground transition-colors hover:border-primary"
          >
            {#if form.login_bg}
              <img src={form.login_bg} alt="登录背景" class="h-full w-full object-cover" />
              <button
                type="button"
                class="absolute right-1 top-1 grid h-5 w-5 place-items-center rounded-full bg-black/60 text-white"
                aria-label="移除登录背景"
                onclick={(e) => { e.preventDefault(); removeImage('login_bg'); }}
              >
                <Icon name="close" size={12} />
              </button>
            {:else}
              <!-- 未上传自定义背景时，展示当前生效的内置默认背景（与登录页一致） -->
              <img src={defaultLoginBg} alt="默认登录背景" class="h-full w-full object-cover" />
              <span class="absolute inset-0 grid place-items-center bg-black/45 text-xs text-white opacity-0 transition-opacity hover:opacity-100">
                <Icon name="upload" size={20} />
              </span>
            {/if}
          </label>
          <input id="c-bg" type="file" accept="image/*" class="hidden" onchange={(e) => pickImage('login_bg', e)} />
        </div>

        <div class={row}>
          <label class={fieldLabel} for="c-icon">产品图标</label>
          <label
            for="c-icon"
            class="relative flex h-[64px] w-[64px] cursor-pointer items-center justify-center overflow-hidden rounded-md border border-input bg-muted/50 text-muted-foreground transition-colors hover:border-primary"
          >
            {#if form.product_icon}
              <img src={form.product_icon} alt="产品图标" class="h-full w-full object-contain" />
              <button
                type="button"
                class="absolute right-0 top-0 grid h-5 w-5 place-items-center rounded-full bg-black/60 text-white"
                aria-label="移除产品图标"
                onclick={(e) => { e.preventDefault(); removeImage('product_icon'); }}
              >
                <Icon name="close" size={12} />
              </button>
            {:else}
              <!-- 未上传自定义图标时，展示当前生效的内置默认 logo（与登录页/顶栏一致） -->
              <img src={defaultLogo} alt="默认产品图标" class="h-full w-full object-contain p-2" />
              <span class="absolute inset-0 grid place-items-center bg-black/45 text-xs text-white opacity-0 transition-opacity hover:opacity-100">
                <Icon name="upload" size={20} />
              </span>
            {/if}
          </label>
          <input id="c-icon" type="file" accept="image/*" class="hidden" onchange={(e) => pickImage('product_icon', e)} />
        </div>

        <!-- 运行时长 | 其它选项 -->
        <div class={row}>
          <label class={fieldLabel} for="c-runtime">运行时长</label>
          <input id="c-runtime" class={inputClass} value={runTime} disabled />
        </div>

        <div class={row}>
          <span class={fieldLabel}>其它选项</span>
          <label for="c-watermark" class="flex h-10 cursor-pointer select-none items-center gap-2 text-sm">
            <input id="c-watermark" type="checkbox" class="h-4 w-4 accent-primary" bind:checked={form.watermark} />
            全局水印
          </label>
        </div>

        <!-- 警示语：单独一行 -->
        <div class={row + ' sm:col-span-2'}>
          <label class={fieldLabel} for="c-warning">警示语</label>
          <input id="c-warning" class={inputClass} bind:value={form.warning_text} />
        </div>

        <!-- 授权时间：单独一行 -->
        <div class={row + ' sm:col-span-2'}>
          <label class={fieldLabel} for="c-license">授权时间</label>
          <input id="c-license" class={inputClass} value={licenseText(config().license_days)} disabled />
        </div>

        <!-- 机器码：单独一行 -->
        <div class={row + ' sm:col-span-2'}>
          <label class={fieldLabel} for="c-machine">机器码</label>
          <input id="c-machine" class={inputClass} value={config().machine_code} disabled />
        </div>

        <!-- 激活码：整行 textarea，最底部 -->
        <div class={row + ' sm:col-span-2'}>
          <label class={fieldLabel} for="c-activation">激活码</label>
          <textarea id="c-activation" class={inputClass + ' min-h-[120px] resize-y'} bind:value={form.activation_code} placeholder="请输入激活码"></textarea>
        </div>
      </div>
    </div>
    <footer class="flex shrink-0 items-center justify-end gap-2 border-t border-border px-5 py-3">
      <Button onclick={save} disabled={busy}>{busy ? '保存中…' : '保存'}</Button>
    </footer>
  </div>
</div>
{/if}
