export function accountStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null || !('response' in error))
    return undefined
  const response = error.response
  if (
    typeof response !== 'object' ||
    response === null ||
    !('status' in response)
  )
    return undefined
  return typeof response.status === 'number' ? response.status : undefined
}

export function accountErrorMessage(error: unknown): string {
  switch (accountStatus(error)) {
    case 400:
    case 422:
      return '请检查输入内容后重试。'
    case 401:
      return '登录信息无效，请检查邮箱和密码。'
    case 403:
      return '当前账户无法执行此操作。'
    case 409:
      return '资料已变化或邮箱已被使用，请核对后重试。'
    case 429:
      return '操作过于频繁，请稍后再试。'
    case 503:
      return '服务暂时不可用，请稍后重试。'
    default:
      return '连接失败，请检查网络后重试。'
  }
}
