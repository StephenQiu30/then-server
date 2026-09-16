// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 读取发现或关注时间流 GET /feed */
export async function listCommunityFeed(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listCommunityFeedParams,
  options?: RequestOptions,
) {
  return request<API.PublicPostPageResponse>('/feed', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      // type has a default value: discover
      type: 'discover',
      ...params,
    },
    ...(options || {}),
  })
}

/** 读取公开主页帖子 GET /profiles/${param0}/posts */
export async function listProfilePosts(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listProfilePostsParams,
  options?: RequestOptions,
) {
  const { handle: param0, ...queryParams } = params
  return request<API.PublicPostPageResponse>(`/profiles/${param0}/posts`, {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 按关键词或标签搜索公开帖子 GET /search/posts */
export async function searchCommunityPosts(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.searchCommunityPostsParams,
  options?: RequestOptions,
) {
  return request<API.PublicPostPageResponse>('/search/posts', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}
