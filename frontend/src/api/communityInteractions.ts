// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 收藏帖子 PUT /posts/${param0}/bookmark */
export async function bookmarkPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.bookmarkPostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}/bookmark`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 取消收藏 DELETE /posts/${param0}/bookmark */
export async function unbookmarkPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.unbookmarkPostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}/bookmark`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 点赞帖子 PUT /posts/${param0}/like */
export async function likePost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.likePostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}/like`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 取消点赞 DELETE /posts/${param0}/like */
export async function unlikePost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.unlikePostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}/like`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 读取本人收藏 GET /users/me/bookmarks */
export async function listBookmarks(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listBookmarksParams,
  options?: RequestOptions,
) {
  return request<API.PublicPostPageResponse>('/users/me/bookmarks', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...params,
    },
    ...(options || {}),
  })
}
