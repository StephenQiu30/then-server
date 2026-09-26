// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 请求本人结构化或原图数据导出 POST /exports */
export async function createDataExport(
  body: API.CreateDataExportInputBody,
  options?: RequestOptions,
) {
  return request<any>('/exports', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 查看本人导出状态 GET /exports/${param0} */
export async function getDataExport(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDataExportParams,
  options?: RequestOptions,
) {
  const { id: param0, ...queryParams } = params
  return request<API.DataExportResponse>(`/exports/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 撤销本人导出 DELETE /exports/${param0} */
export async function revokeDataExport(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.revokeDataExportParams,
  options?: RequestOptions,
) {
  const { id: param0, ...queryParams } = params
  return request<any>(`/exports/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 下载本人私有 ZIP GET /exports/${param0}/content */
export async function downloadDataExport(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.downloadDataExportParams,
  options?: RequestOptions,
) {
  const { id: param0, ...queryParams } = params
  return request<string>(`/exports/${param0}/content`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}
