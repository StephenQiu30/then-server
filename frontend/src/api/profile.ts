// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 按唯一标识读取公开资料 GET /profiles/${param0} */
export async function getPublicProfile(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPublicProfileParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<API.PublicProfileResponse>(`/profiles/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 读取公开资料当前净化头像 GET /profiles/${param0}/avatar */
export async function getPublicProfileAvatar(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPublicProfileAvatarParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<string>(`/profiles/${param0}/avatar`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 获取本人公开资料 GET /users/me/profile */
export async function getCurrentProfile(options?: RequestOptions) {
  return request<API.PublicProfileResponse>('/users/me/profile', {
    method: 'GET',
    ...(options || {}),
  })
}

/** 创建或修改本人公开资料 PUT /users/me/profile */
export async function putCurrentProfile(
  body: API.PutProfileRequest,
  options?: RequestOptions,
) {
  return request<API.PublicProfileResponse>('/users/me/profile', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 绑定本人净化头像并撤下旧头像 PUT /users/me/profile/avatar */
export async function putProfileAvatar(
  body: API.PutProfileAvatarRequest,
  options?: RequestOptions,
) {
  return request<API.PublicProfileResponse>('/users/me/profile/avatar', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 撤下本人头像并清理旧图片 DELETE /users/me/profile/avatar */
export async function deleteProfileAvatar(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteProfileAvatarParams,
  options?: RequestOptions,
) {
  return request<API.PublicProfileResponse>('/users/me/profile/avatar', {
    method: 'DELETE',
    params: {
      ...params,
    },
    ...(options || {}),
  })
}
