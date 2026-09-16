// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 按本地日期稳定分页列出本人实际穿着 GET /wear-events */
export async function listWearEvents(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listWearEventsParams,
  options?: RequestOptions,
) {
  return request<API.WearEventPageResponse>('/wear-events', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}

/** 记录本人实际穿着并生成服务端快照 POST /wear-events */
export async function createWearEvent(
  body: API.CreateWearEventRequest,
  options?: RequestOptions,
) {
  return request<API.WearEventResponse>('/wear-events', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 读取本人实际穿着与快照 GET /wear-events/${param0} */
export async function getWearEvent(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getWearEventParams,
  options?: RequestOptions,
) {
  const { wear_event_id: param0, ...queryParams } = params
  return request<API.WearEventResponse>(`/wear-events/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 按 revision 纠正实际穿着 PUT /wear-events/${param0} */
export async function updateWearEvent(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.updateWearEventParams,
  body: API.UpdateWearEventRequest,
  options?: RequestOptions,
) {
  const { wear_event_id: param0, ...queryParams } = params
  return request<API.WearEventResponse>(`/wear-events/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 永久删除实际穿着并重算计划状态 DELETE /wear-events/${param0} */
export async function deleteWearEvent(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteWearEventParams,
  options?: RequestOptions,
) {
  const { wear_event_id: param0, ...queryParams } = params
  return request<any>(`/wear-events/${param0}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}
