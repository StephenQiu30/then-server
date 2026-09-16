// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../request'

/** 获取当前本人成年声明状态 GET /privacy/self-adult-declaration */
export async function getSelfAdultDeclaration(options?: RequestOptions) {
  return request<API.SelfAdultDeclarationResponse>(
    '/privacy/self-adult-declaration',
    {
      method: 'GET',
      ...(options || {}),
    },
  )
}

/** 确认当前本人成年声明 PUT /privacy/self-adult-declaration */
export async function confirmSelfAdultDeclaration(
  body: API.ConfirmSelfAdultDeclarationRequest,
  options?: RequestOptions,
) {
  return request<API.SelfAdultDeclarationResponse>(
    '/privacy/self-adult-declaration',
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      data: body,
      ...(options || {}),
    },
  )
}

/** 撤回当前本人成年声明 DELETE /privacy/self-adult-declaration */
export async function withdrawSelfAdultDeclaration(options?: RequestOptions) {
  return request<API.SelfAdultDeclarationResponse>(
    '/privacy/self-adult-declaration',
    {
      method: 'DELETE',
      ...(options || {}),
    },
  )
}
