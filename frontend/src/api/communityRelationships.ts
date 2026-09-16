// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 关注用户 PUT /profiles/${param0}/follow */
export async function followProfile(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.followProfileParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<any>(`/profiles/${param0}/follow`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 取消关注 DELETE /profiles/${param0}/follow */
export async function unfollowProfile(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.unfollowProfileParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<any>(`/profiles/${param0}/follow`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 读取粉丝列表 GET /profiles/${param0}/followers */
export async function listProfileFollowers(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listProfileFollowersParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<API.PublicProfilePageResponse>(
    `/profiles/${param0}/followers`,
    {
      method: 'GET',
      params: {
        // limit has a default value: 20
        limit: '20',
        ...queryParams,
      },
      ...(options || {}),
    },
  )
}

/** 读取关注列表 GET /profiles/${param0}/following */
export async function listProfileFollowing(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listProfileFollowingParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<API.PublicProfilePageResponse>(
    `/profiles/${param0}/following`,
    {
      method: 'GET',
      params: {
        // limit has a default value: 20
        limit: '20',
        ...queryParams,
      },
      ...(options || {}),
    },
  )
}
