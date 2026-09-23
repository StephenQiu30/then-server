import { accountStatus } from '@/lib/account/errors'

export function wardrobeErrorMessage(error: unknown): string {
  switch (accountStatus(error)) {
    case 400:
    case 422:
      return '请检查衣物信息后重试。'
    case 403:
      return '当前账户无法修改这件衣物。'
    case 404:
      return '这件衣物已不存在，请刷新列表。'
    case 409:
      return '衣物或关联记录已变化，请重新加载后确认。'
    case 503:
      return '服务暂时不可用，请稍后重试。'
    default:
      return '连接失败，请检查网络后重试。'
  }
}
