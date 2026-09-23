// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 从本人现存实际穿着实时计算区间统计 GET /statistics/wear */
export async function getWearStatistics(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getWearStatisticsParams,
  options?: RequestOptions,
) {
  return request<API.WearStatisticsResponse>('/statistics/wear', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  })
}

/** 读取本人实际穿着的明确反馈 GET /wear-events/${param0}/feedback */
export async function getWearFeedback(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getWearFeedbackParams,
  options?: RequestOptions,
) {
  const { wear_event_id: param0, ...queryParams } = params
  return request<API.FeedbackResponse>(`/wear-events/${param0}/feedback`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 按 revision 保存或纠正明确反馈 PUT /wear-events/${param0}/feedback */
export async function saveWearFeedback(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.saveWearFeedbackParams,
  body: API.SaveFeedbackRequest,
  options?: RequestOptions,
) {
  const { wear_event_id: param0, ...queryParams } = params
  return request<API.FeedbackResponse>(`/wear-events/${param0}/feedback`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 按 revision 撤回反馈 DELETE /wear-events/${param0}/feedback */
export async function deleteWearFeedback(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteWearFeedbackParams,
  options?: RequestOptions,
) {
  const { wear_event_id: param0, ...queryParams } = params
  return request<any>(`/wear-events/${param0}/feedback`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}
