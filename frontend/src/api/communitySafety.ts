// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 读取本人屏蔽列表 GET /users/me/blocks */
export async function listBlockedUsers(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listBlockedUsersParams,
  options?: RequestOptions,
) {
  return request<API.BlockedUserPageResponse>('/users/me/blocks', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...params,
    },
    ...(options || {}),
  })
}

/** 屏蔽用户并解除双向关注 PUT /users/me/blocks/${param0} */
export async function blockUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.blockUserParams,
  options?: RequestOptions,
) {
  const { user_id: param0, ...queryParams } = params
  return request<any>(`/users/me/blocks/${param0}`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 取消屏蔽用户 DELETE /users/me/blocks/${param0} */
export async function unblockUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.unblockUserParams,
  options?: RequestOptions,
) {
  const { user_id: param0, ...queryParams } = params
  return request<any>(`/users/me/blocks/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}
