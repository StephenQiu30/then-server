// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 匿名读取当前批准帖子版本 GET /posts/${param0} */
export async function getPublicPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPublicPostParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<API.PublicPostResponse>(`/posts/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 匿名读取当前批准版本图片 GET /posts/${param0}/images/${param1} */
export async function getPublicPostImage(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPublicPostImageParams,
  options?: RequestOptions,
) {
  const { post_id: param0, ordinal: param1, ...queryParams } = params
  return request<string>(`/posts/${param0}/images/${param1}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}
