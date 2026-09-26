// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 按本人提交顺序分页读取结构化变更 GET /sync/changes */
export async function listSyncChanges(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listSyncChangesParams,
  options?: RequestOptions,
) {
  return request<API.SyncChangesResponse>('/sync/changes', {
    method: 'GET',
    params: {
      // limit has a default value: 50
      limit: '50',
      ...params,
    },
    ...(options || {}),
  })
}
