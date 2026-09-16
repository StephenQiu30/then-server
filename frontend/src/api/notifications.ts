// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 读取已投递通知 GET /notifications */
export async function listNotifications(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listNotificationsParams,
  options?: RequestOptions,
) {
  return request<API.NotificationPageResponse>('/notifications', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...params,
    },
    ...(options || {}),
  })
}

/** 标记通知已读 PUT /notifications/${param0}/read */
export async function markNotificationRead(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.markNotificationReadParams,
  options?: RequestOptions,
) {
  const { notification_id: param0, ...queryParams } = params
  return request<API.NotificationResponse>(`/notifications/${param0}/read`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  })
}
