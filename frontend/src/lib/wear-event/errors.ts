import { accountStatus } from '../account/errors.ts'

export function wearErrorMessage(error: unknown): string {
  switch (accountStatus(error)) {
    case 400:
    case 422:
      return '请检查日期、来源计划、衣物和确认信息。'
    case 403:
      return '当前账户无法修改这条记录。'
    case 404:
      return '记录已不存在，请刷新列表。'
    case 409:
      return '记录、计划或衣物已变化。请核对列表并重新加载记录。'
    case 429:
      return '操作过于频繁，请稍后重试。'
    case 503:
      return '服务暂时不可用，请稍后重试。'
    default:
      return '连接失败，请检查网络后重试。'
  }
}

export function duplicateCandidates(
  error: unknown,
): API.WearEventCandidateResponse[] | null {
  if (
    accountStatus(error) !== 409 ||
    typeof error !== 'object' ||
    error === null ||
    !('response' in error)
  )
    return null
  const response = error.response
  if (
    typeof response !== 'object' ||
    response === null ||
    !('data' in response)
  )
    return null
  const data = response.data
  if (
    typeof data !== 'object' ||
    data === null ||
    !('duplicate_candidates' in data)
  )
    return null
  const candidates = data.duplicate_candidates
  if (
    !Array.isArray(candidates) ||
    candidates.length === 0 ||
    candidates.length > 50
  )
    return null
  if (
    !candidates.every(
      (value) =>
        typeof value === 'object' &&
        value !== null &&
        typeof value.id === 'string' &&
        typeof value.revision === 'number',
    )
  )
    return null
  return candidates.map(({ id, revision }) => ({ id, revision }))
}
