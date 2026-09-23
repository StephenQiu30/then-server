import { accountStatus } from '@/lib/account/errors'

export function planErrorMessage(error: unknown): string {
  switch (accountStatus(error)) {
    case 400:
    case 422:
      return '请检查日期、衣物和确认信息后重试。'
    case 403:
      return '当前账户无法修改这条计划。'
    case 404:
      return '这条计划已不存在，请刷新列表。'
    case 409:
      return '计划或衣物已变化。请重新加载计划；若衣物版本已更新，取消选择后重新勾选。'
    case 503:
      return '服务暂时不可用，请稍后重试。'
    default:
      return '连接失败，请检查网络后重试。'
  }
}
