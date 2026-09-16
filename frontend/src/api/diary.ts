// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 按原本地日期聚合计划、实际穿着和日记 GET /calendar */
export async function getCalendarMonth(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getCalendarMonthParams,
  options?: RequestOptions,
) {
  return request<API.CalendarMonthResponse>('/calendar', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  })
}

/** 稳定分页列出本人日记 GET /diary-entries */
export async function listDiaryEntries(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listDiaryEntriesParams,
  options?: RequestOptions,
) {
  return request<API.DiaryEntryPageResponse>('/diary-entries', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}

/** 创建本人私人穿搭日记 POST /diary-entries */
export async function createDiaryEntry(
  body: API.CreateDiaryEntryRequest,
  options?: RequestOptions,
) {
  return request<API.DiaryEntryResponse>('/diary-entries', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 读取本人私人日记 GET /diary-entries/${param0} */
export async function getDiaryEntry(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDiaryEntryParams,
  options?: RequestOptions,
) {
  const { entry_id: param0, ...queryParams } = params
  return request<API.DiaryEntryResponse>(`/diary-entries/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 按 revision 完整更新本人日记 PUT /diary-entries/${param0} */
export async function updateDiaryEntry(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.updateDiaryEntryParams,
  body: API.UpdateDiaryEntryRequest,
  options?: RequestOptions,
) {
  const { entry_id: param0, ...queryParams } = params
  return request<API.DiaryEntryResponse>(`/diary-entries/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 永久删除日记并保留独立事实 DELETE /diary-entries/${param0} */
export async function deleteDiaryEntry(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteDiaryEntryParams,
  options?: RequestOptions,
) {
  const { entry_id: param0, ...queryParams } = params
  return request<any>(`/diary-entries/${param0}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 查看日记删除影响 GET /diary-entries/${param0}/deletion-impact */
export async function getDiaryEntryDeletionImpact(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDiaryEntryDeletionImpactParams,
  options?: RequestOptions,
) {
  const { entry_id: param0, ...queryParams } = params
  return request<API.DiaryDeletionImpactResponse>(
    `/diary-entries/${param0}/deletion-impact`,
    {
      method: 'GET',
      params: { ...queryParams },
      ...(options || {}),
    },
  )
}
