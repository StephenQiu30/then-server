// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 分页列出本人结构化衣物 GET /wardrobe/items */
export async function listWardrobeItems(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listWardrobeItemsParams,
  options?: RequestOptions,
) {
  return request<API.WardrobePageResponse>('/wardrobe/items', {
    method: 'GET',
    params: {
      // limit has a default value: 50
      limit: '50',

      // lifecycle has a default value: active
      lifecycle: 'active',
      ...params,
    },
    ...(options || {}),
  })
}

/** 创建本人结构化衣物与确认属性 POST /wardrobe/items */
export async function createWardrobeItem(
  body: API.CreateWardrobeItemRequest,
  options?: RequestOptions,
) {
  return request<API.WardrobeItemResponse>('/wardrobe/items', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 读取本人结构化衣物 GET /wardrobe/items/${param0} */
export async function getWardrobeItem(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getWardrobeItemParams,
  options?: RequestOptions,
) {
  const { item_id: param0, ...queryParams } = params
  return request<API.WardrobeItemResponse>(`/wardrobe/items/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 按 revision 修改本人结构化衣物 PUT /wardrobe/items/${param0} */
export async function updateWardrobeItem(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.updateWardrobeItemParams,
  body: API.UpdateWardrobeItemRequest,
  options?: RequestOptions,
) {
  const { item_id: param0, ...queryParams } = params
  return request<API.WardrobeItemResponse>(`/wardrobe/items/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 按 revision 删除本人结构化衣物 DELETE /wardrobe/items/${param0} */
export async function deleteWardrobeItem(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteWardrobeItemParams,
  options?: RequestOptions,
) {
  const { item_id: param0, ...queryParams } = params
  return request<any>(`/wardrobe/items/${param0}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  })
}

/** 按 revision 归档本人衣物 POST /wardrobe/items/${param0}/archive */
export async function archiveWardrobeItem(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.archiveWardrobeItemParams,
  body: API.WardrobeLifecycleRequest,
  options?: RequestOptions,
) {
  const { item_id: param0, ...queryParams } = params
  return request<API.WardrobeItemResponse>(
    `/wardrobe/items/${param0}/archive`,
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

/** 读取衣物删除对计划的当前影响 GET /wardrobe/items/${param0}/deletion-impact */
export async function getWardrobeDeletionImpact(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getWardrobeDeletionImpactParams,
  options?: RequestOptions,
) {
  const { item_id: param0, ...queryParams } = params
  return request<API.WardrobeDeletionImpactResponse>(
    `/wardrobe/items/${param0}/deletion-impact`,
    {
      method: 'GET',
      params: { ...queryParams },
      ...(options || {}),
    },
  )
}

/** 按 revision 恢复本人衣物 POST /wardrobe/items/${param0}/restore */
export async function restoreWardrobeItem(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.restoreWardrobeItemParams,
  body: API.WardrobeLifecycleRequest,
  options?: RequestOptions,
) {
  const { item_id: param0, ...queryParams } = params
  return request<API.WardrobeItemResponse>(
    `/wardrobe/items/${param0}/restore`,
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

/** 按本人真实可用衣橱和确认约束推荐穿搭 POST /wardrobe/recommendations */
export async function recommendWardrobeOutfits(
  body: API.WardrobeRecommendationRequest,
  options?: RequestOptions,
) {
  return request<API.WardrobeRecommendationResponse>(
    '/wardrobe/recommendations',
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      data: body,
      ...(options || {}),
    },
  )
}
