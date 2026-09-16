// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 删除本人评论并保留墓碑 DELETE /comments/${param0} */
export async function deleteComment(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteCommentParams,
  options?: RequestOptions,
) {
  const { comment_id: param0, ...queryParams } = params
  return request<any>(`/comments/${param0}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 读取一级回复 GET /comments/${param0}/replies */
export async function listCommentReplies(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listCommentRepliesParams,
  options?: RequestOptions,
) {
  const { comment_id: param0, ...queryParams } = params
  return request<API.CommentPageResponse>(`/comments/${param0}/replies`, {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 读取公开评论或回复 GET /posts/${param0}/comments */
export async function listComments(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listCommentsParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<API.CommentPageResponse>(`/posts/${param0}/comments`, {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 创建待审评论或回复 POST /posts/${param0}/comments */
export async function createComment(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.createCommentParams,
  body: API.CreateCommentRequest,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/posts/${param0}/comments`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}
