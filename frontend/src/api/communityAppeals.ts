// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 管理员读取申诉 GET /admin/moderation-appeals */
export async function listModerationAppeals(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listModerationAppealsParams,
  options?: RequestOptions,
) {
  return request<API.AdminAppealPageResponse>('/admin/moderation-appeals', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}

/** 管理员解决申诉 POST /admin/moderation-appeals/${param0}/resolve */
export async function resolveModerationAppeal(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.resolveModerationAppealParams,
  body: API.ResolveAppealRequest,
  options?: RequestOptions,
) {
  const { appeal_id: param0, ...queryParams } = params
  return request<API.AppealResponse>(
    `/admin/moderation-appeals/${param0}/resolve`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      params: { ...queryParams },
      data: body,
      ...(options || {}),
    },
  )
}

/** 申诉关联治理动作 POST /moderation-appeals */
export async function createModerationAppeal(
  body: API.CreateAppealRequest,
  options?: RequestOptions,
) {
  return request<API.AppealResponse>('/moderation-appeals', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 读取本人申诉 GET /users/me/moderation-appeals */
export async function listOwnModerationAppeals(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listOwnModerationAppealsParams,
  options?: RequestOptions,
) {
  return request<API.AppealPageResponse>('/users/me/moderation-appeals', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...params,
    },
    ...(options || {}),
  })
}
