// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../request'

/** 同意本人照片的固定本地开发用途 POST /consents */
export async function createConsent(
  body: API.CreateConsentRequest,
  options?: RequestOptions,
) {
  return request<API.ConsentResponse>('/consents', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 获取本人同意状态 GET /consents/${param0} */
export async function getConsent(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getConsentParams,
  options?: RequestOptions,
) {
  const { consent_id: param0, ...queryParams } = params
  return request<API.ConsentResponse>(`/consents/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 撤回本人照片同意 POST /consents/${param0}/withdraw */
export async function withdrawConsent(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.withdrawConsentParams,
  options?: RequestOptions,
) {
  const { consent_id: param0, ...queryParams } = params
  return request<API.ConsentResponse>(`/consents/${param0}/withdraw`, {
    method: 'POST',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 获取本人媒体删除进度 GET /deletion-requests/${param0} */
export async function getDeletionRequest(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDeletionRequestParams,
  options?: RequestOptions,
) {
  const { request_id: param0, ...queryParams } = params
  return request<API.DeletionRequestResponse>(`/deletion-requests/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 获取本人媒体处理状态 GET /media/${param0} */
export async function getMedia(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getMediaParams,
  options?: RequestOptions,
) {
  const { media_id: param0, ...queryParams } = params
  return request<API.MediaResponse>(`/media/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 立即撤销读取并请求删除媒体 DELETE /media/${param0} */
export async function deleteMedia(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteMediaParams,
  options?: RequestOptions,
) {
  const { media_id: param0, ...queryParams } = params
  return request<any>(`/media/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 固定已上传对象版本 POST /media/${param0}/complete */
export async function completeMediaUpload(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.completeMediaUploadParams,
  body: API.CompleteMediaUploadRequest,
  options?: RequestOptions,
) {
  const { media_id: param0, ...queryParams } = params
  return request<API.MediaResponse>(`/media/${param0}/complete`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 创建一次 JPEG 私有直传意图 POST /media/uploads */
export async function createMediaUpload(
  body: API.CreateMediaUploadRequest,
  options?: RequestOptions,
) {
  return request<API.MediaUploadResponse>('/media/uploads', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}
