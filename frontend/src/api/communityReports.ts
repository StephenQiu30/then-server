// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 举报当前公开帖子 POST /reports */
export async function createPostReport(
  body: API.CreateReportRequest,
  options?: RequestOptions,
) {
  return request<API.ContentReportResponse>('/reports', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 分页读取本人举报处理状态 GET /users/me/reports */
export async function listOwnReports(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listOwnReportsParams,
  options?: RequestOptions,
) {
  return request<API.ReportPageResponse>('/users/me/reports', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}
