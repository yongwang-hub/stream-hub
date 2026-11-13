// 类名拼接：组件里按条件拼 Tailwind 类用，避免引入 clsx/tailwind-merge 这类运行时依赖。
export function cn(
  ...classes: Array<string | false | null | undefined>
): string {
  return classes.filter(Boolean).join(' ');
}
