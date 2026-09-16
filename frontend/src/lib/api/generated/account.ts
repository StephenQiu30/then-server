// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../request'

/** 获取本人账户 GET /users/me */
export async function getCurrentUser(options?: RequestOptions) {
  return request<API.UserResponse>('/users/me', {
    method: 'GET',
    ...(options || {}),
  })
}

/** 删除本人账户及全部会话 存在未完成删除的私有媒体时返回 409，避免数据库级联留下孤立对象。 DELETE /users/me */
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
