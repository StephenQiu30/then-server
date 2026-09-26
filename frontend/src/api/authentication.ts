// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 为当前邮箱申请验证邮件 POST /auth/email-verifications */
export async function requestEmailVerification(options?: RequestOptions) {
  return request<any>('/auth/email-verifications', {
    method: 'POST',
    ...(options || {}),
  })
}

/** 确认当前邮箱的单次验证挑战 POST /auth/email-verifications/confirm */
export async function confirmEmailVerification(
  body: API.ConfirmMailChallengeInputBody,
  options?: RequestOptions,
) {
  return request<any>('/auth/email-verifications/confirm', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 申请密码找回邮件 POST /auth/password-resets */
export async function requestPasswordReset(
  body: API.RequestPasswordResetInputBody,
  options?: RequestOptions,
) {
  return request<any>('/auth/password-resets', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 用单次邮件挑战重置密码并撤销旧会话 POST /auth/password-resets/confirm */
export async function confirmPasswordReset(
  body: API.ConfirmPasswordResetInputBody,
  options?: RequestOptions,
) {
  return request<any>('/auth/password-resets/confirm', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

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
