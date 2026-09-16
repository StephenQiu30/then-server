// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 创建本人私有帖子草稿 POST /posts */
export async function createPost(
  body: API.CreatePostRequest,
  options?: RequestOptions,
) {
  return request<API.PostResponse>('/posts', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 创建新的私有帖子草稿版本 PUT /posts/${param0} */
export async function updatePost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.updatePostParams,
  body: API.UpdatePostRequest,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<API.PostResponse>(`/posts/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 删除帖子内容并保留最小墓碑 DELETE /posts/${param0} */
export async function deletePost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deletePostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 明确提交当前草稿进入人工审核 POST /posts/${param0}/submit */
export async function submitPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.submitPostParams,
  body: API.PostRevisionRequest,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}/submit`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 撤回公开或待审帖子 POST /posts/${param0}/withdraw */
export async function withdrawPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.withdrawPostParams,
  body: API.PostRevisionRequest,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<API.PostResponse>(`/posts/${param0}/withdraw`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 分页读取本人全部帖子状态 GET /users/me/posts */
export async function listOwnPosts(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listOwnPostsParams,
  options?: RequestOptions,
) {
  return request<API.PostPageResponse>('/users/me/posts', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}

/** 读取本人帖子及当前私有版本 GET /users/me/posts/${param0} */
export async function getOwnPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getOwnPostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<API.PostResponse>(`/users/me/posts/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}
