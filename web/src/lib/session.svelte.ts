// 当前登录用户名：只存内存（和 token 一致，刷新回登录页），供水印与顶栏展示用。
// 参考 Vue 版：水印内容是 `${userStore.userInfo?.userName}  ${currentTime}`。
let name = $state('admin');

export function currentUser(): string {
  return name;
}

export function setCurrentUser(value: string): void {
  name = value;
}
