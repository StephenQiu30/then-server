// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 注销后查询删除回执 GET /account-deletion-requests/${param0} */
export async function getAccountDeletionReceipt(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getAccountDeletionReceiptParams,
  options?: RequestOptions,
) {
  const { id: param0, ...queryParams } = params
  return request<API.AccountDeletionReceiptResponse>(
    `/account-deletion-requests/${param0}`,
    {
      method: 'GET',
      params: { ...queryParams },
      ...(options || {}),
    },
  )
}

/** 撤销删除回执查询权 DELETE /account-deletion-requests/${param0} */
export async function revokeAccountDeletionReceipt(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.revokeAccountDeletionReceiptParams,
  options?: RequestOptions,
) {
  const { id: param0, ...queryParams } = params
  return request<any>(`/account-deletion-requests/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 获取本人账户 GET /users/me */
export async function getCurrentUser(options?: RequestOptions) {
  return request<API.UserResponse>('/users/me', {
    method: 'GET',
    ...(options || {}),
  })
}

/** 受理本人账户删除 立即撤销全部会话并关闭公开内容；对象存储清理完成后物理删除账户。 DELETE /users/me */
export async function deleteCurrentUser(options?: RequestOptions) {
  return request<any>('/users/me', {
    method: 'DELETE',
    ...(options || {}),
  })
}

/** 修改本人邮箱或显示名称 PATCH /users/me */
export async function updateCurrentUser(
  body: {
    /** 新显示名称 */
    display_name?: string
    /** 新登录邮箱 */
    email?: string
    /** 从上次读取结果取得的账户版本；不匹配时返回 409 */
    expected_revision: number
  },
  options?: RequestOptions,
) {
  return request<API.UserResponse>('/users/me', {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 分页列出本人有效会话 GET /users/me/sessions */
export async function listCurrentUserSessions(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listCurrentUserSessionsParams,
  options?: RequestOptions,
) {
  return request<API.SessionPageResponse>('/users/me/sessions', {
    method: 'GET',
    params: {
      // limit has a default value: 50
      limit: '50',
      ...params,
    },
    ...(options || {}),
  })
}

/** 撤销本人指定会话 DELETE /users/me/sessions/${param0} */
export async function revokeCurrentUserSession(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.revokeCurrentUserSessionParams,
  options?: RequestOptions,
) {
  const { session_id: param0, ...queryParams } = params
  return request<any>(`/users/me/sessions/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}
