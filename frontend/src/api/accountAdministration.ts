// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 管理员分页读取账号状态 GET /admin/users */
export async function listAdminUsers(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listAdminUsersParams,
  options?: RequestOptions,
) {
  return request<API.AdminUserPageResponse>('/admin/users', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...params,
    },
    ...(options || {}),
  })
}

/** 恢复账号，不改变帖子治理状态 POST /admin/users/${param0}/restore */
export async function restoreUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.restoreUserParams,
  body: API.SetUserStatusRequest,
  options?: RequestOptions,
) {
  const { user_id: param0, ...queryParams } = params
  return request<API.AdminUserResponse>(`/admin/users/${param0}/restore`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 封禁账号并撤销全部会话 POST /admin/users/${param0}/suspend */
export async function suspendUser(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.suspendUserParams,
  body: API.SetUserStatusRequest,
  options?: RequestOptions,
) {
  const { user_id: param0, ...queryParams } = params
  return request<API.AdminUserResponse>(`/admin/users/${param0}/suspend`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}
