// 统一的表单/表格样式常量：页面用原生标签 + 这些类，避免每个组件重复写 Tailwind 类名。
// 颜色全部引用 app.css 的语义 token；字号取 app.css @theme 的全局刻度。

export const inputClass =
  'w-full rounded-lg border border-input bg-background px-3.5 py-2.5 text-sm text-foreground placeholder:text-muted-foreground transition-colors focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/20 disabled:cursor-not-allowed disabled:opacity-60';

export const labelClass =
  'block text-xs font-medium text-muted-foreground mb-2';

export const panelClass =
  'rounded-xl border border-border bg-card text-card-foreground shadow-sm';

export const tableWrapClass = 'overflow-x-auto ui-scroll';

export const tableClass = 'w-full border-collapse text-sm';

export const thClass =
  'border-b border-border px-4 py-3.5 text-left text-xs font-semibold uppercase tracking-wide text-muted-foreground whitespace-nowrap';

export const tdClass =
  'border-b border-border px-4 py-3.5 text-sm text-foreground align-middle';

export const rowHoverClass = 'transition-colors hover:bg-muted/60';
