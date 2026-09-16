// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

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
