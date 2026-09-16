// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 分页读取最小治理审计动作 GET /admin/moderation-actions */
export async function listModerationActions(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listModerationActionsParams,
  options?: RequestOptions,
) {
  return request<API.ModerationActionPageResponse>(
    '/admin/moderation-actions',
    {
      method: 'GET',
      params: {
        // limit has a default value: 20
        limit: '20',
        ...params,
      },
      ...(options || {}),
    },
  )
}

/** 读取待审帖子队列 GET /admin/moderation/posts */
export async function listPostModerationCandidates(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listPostModerationCandidatesParams,
  options?: RequestOptions,
) {
  return request<API.ModerationCandidatePageResponse>(
    '/admin/moderation/posts',
    {
      method: 'GET',
      params: {
        // limit has a default value: 20
        limit: '20',
        ...params,
      },
      ...(options || {}),
    },
  )
}

/** 按版本、轮次和 revision 决定帖子审核 POST /admin/moderation/posts/${param0}/decisions */
export async function decidePostModeration(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.decidePostModerationParams,
  body: API.DecidePostRequest,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<API.PostResponse>(
    `/admin/moderation/posts/${param0}/decisions`,
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

/** 读取一个待审帖子版本 GET /admin/moderation/posts/${param0}/revisions/${param1} */
export async function getPostModerationCandidate(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPostModerationCandidateParams,
  options?: RequestOptions,
) {
  const { post_id: param0, version: param1, ...queryParams } = params
  return request<API.ModerationCandidateResponse>(
    `/admin/moderation/posts/${param0}/revisions/${param1}`,
    {
      method: 'GET',
      params: { ...queryParams },
      ...(options || {}),
    },
  )
}

/** 读取待审版本净化图片 GET /admin/moderation/posts/${param0}/revisions/${param1}/images/${param2} */
export async function getPostModerationImage(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPostModerationImageParams,
  options?: RequestOptions,
) {
  const {
    post_id: param0,
    version: param1,
    ordinal: param2,
    ...queryParams
  } = params
  return request<string>(
    `/admin/moderation/posts/${param0}/revisions/${param1}/images/${param2}`,
    {
      method: 'GET',
      params: { ...queryParams },
      ...(options || {}),
    },
  )
}

/** 独立下架帖子并记录治理动作 POST /admin/posts/${param0}/remove */
export async function removePublishedPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.removePublishedPostParams,
  body: API.RemovePostRequest,
  options?: RequestOptions,
) {
  const { post_id: param0, ...queryParams } = params
  return request<any>(`/admin/posts/${param0}/remove`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  })
}

/** 运营分页读取举报 GET /admin/reports */
export async function listCommunityReports(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listCommunityReportsParams,
  options?: RequestOptions,
) {
  return request<API.AdminReportPageResponse>('/admin/reports', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',

      ...params,
    },
    ...(options || {}),
  })
}

/** 解决或驳回举报，不自动下架 POST /admin/reports/${param0}/resolve */
export async function resolveCommunityReport(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.resolveCommunityReportParams,
  body: API.ResolveReportRequest,
  options?: RequestOptions,
) {
  const { report_id: param0, ...queryParams } = params
  return request<API.ContentReportResponse>(
    `/admin/reports/${param0}/resolve`,
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
