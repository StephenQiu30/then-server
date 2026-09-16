// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 注册账户并建立会话 POST /auth/registrations */
export async function registerAccount(
  body: API.RegisterAccountRequest,
  options?: RequestOptions,
) {
  return request<API.AuthenticatedUserResponse>('/auth/registrations', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 退出当前会话 DELETE /auth/session */
export async function deleteSession(options?: RequestOptions) {
  return request<any>('/auth/session', {
    method: 'DELETE',
    ...(options || {}),
  })
}

/** 使用邮箱密码登录 POST /auth/sessions */
export async function createSession(
  body: API.CreateSessionRequest,
  options?: RequestOptions,
) {
  return request<API.AuthenticatedUserResponse>('/auth/sessions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}
