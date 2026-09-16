// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../request'

/** 按本地日期稳定分页列出本人计划 GET /outfit-plans */
export async function listOutfitPlans(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listOutfitPlansParams,
  options?: RequestOptions,
) {
  return request<API.OutfitPlanPageResponse>('/outfit-plans', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}

/** 创建本人真实衣物穿搭计划 POST /outfit-plans */
export async function createOutfitPlan(
  body: API.CreateOutfitPlanRequest,
  options?: RequestOptions,
) {
  return request<API.OutfitPlanResponse>('/outfit-plans', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 读取本人穿搭计划与服务端快照 GET /outfit-plans/${param0} */
export async function getOutfitPlan(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getOutfitPlanParams,
  options?: RequestOptions,
) {
  const { plan_id: param0, ...queryParams } = params
  return request<API.OutfitPlanResponse>(`/outfit-plans/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 按 revision 完整更新 active 计划 PUT /outfit-plans/${param0} */
export async function updateOutfitPlan(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.updateOutfitPlanParams,
  body: API.UpdateOutfitPlanRequest,
  options?: RequestOptions,
) {
  const { plan_id: param0, ...queryParams } = params
  return request<API.OutfitPlanResponse>(`/outfit-plans/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 永久删除计划并阻止迟到复活 DELETE /outfit-plans/${param0} */
export async function deleteOutfitPlan(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteOutfitPlanParams,
  options?: RequestOptions,
) {
  const { plan_id: param0, ...queryParams } = params
  return request<any>(`/outfit-plans/${param0}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 取消计划且不创建实际穿着 POST /outfit-plans/${param0}/cancel */
export async function cancelOutfitPlan(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.cancelOutfitPlanParams,
  body: API.CancelOutfitPlanRequest,
  options?: RequestOptions,
) {
  const { plan_id: param0, ...queryParams } = params
  return request<API.OutfitPlanResponse>(`/outfit-plans/${param0}/cancel`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 确认计划最终未穿 POST /outfit-plans/${param0}/not-worn */
export async function markOutfitPlanNotWorn(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.markOutfitPlanNotWornParams,
  body: API.TransitionOutfitPlanRequest,
  options?: RequestOptions,
) {
  const { plan_id: param0, ...queryParams } = params
  return request<API.OutfitPlanResponse>(`/outfit-plans/${param0}/not-worn`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 把未穿计划恢复为待确认 POST /outfit-plans/${param0}/restore */
export async function restoreOutfitPlan(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.restoreOutfitPlanParams,
  body: API.TransitionOutfitPlanRequest,
  options?: RequestOptions,
) {
  const { plan_id: param0, ...queryParams } = params
  return request<API.OutfitPlanResponse>(`/outfit-plans/${param0}/restore`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}
