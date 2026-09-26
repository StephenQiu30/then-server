// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../lib/api/request'

/** 管理员依据证据核对未知提交结果 POST /admin/generation-jobs/${param0}/submission-reconciliation */
export async function reconcileUnknownGenerationSubmission(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.reconcileUnknownGenerationSubmissionParams,
  body: API.ReconcileUnknownSubmissionRequest,
  options?: RequestOptions,
) {
  const { job_id: param0, ...queryParams } = params
  return request<API.SubmissionReconciliationResponse>(
    `/admin/generation-jobs/${param0}/submission-reconciliation`,
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

/** 管理员列出待核对的生成提交 GET /admin/generation/submission-reconciliations */
export async function listUnknownGenerationSubmissions(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listUnknownGenerationSubmissionsParams,
  options?: RequestOptions,
) {
  return request<API.UnknownSubmissionPageResponse>(
    '/admin/generation/submission-reconciliations',
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

/** 列出本人生成任务 GET /generation-jobs */
export async function listGenerationJobs(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listGenerationJobsParams,
  options?: RequestOptions,
) {
  return request<API.GenerationJobPageResponse>('/generation-jobs', {
    method: 'GET',
    params: {
      // limit has a default value: 20
      limit: '20',
      ...params,
    },
    ...(options || {}),
  })
}

/** 接纳一次 provider 无关的生成任务 POST /generation-jobs */
export async function createGenerationJob(
  body: API.CreateGenerationJobRequest,
  options?: RequestOptions,
) {
  return request<any>('/generation-jobs', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  })
}

/** 获取本人生成任务 GET /generation-jobs/${param0} */
export async function getGenerationJob(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getGenerationJobParams,
  options?: RequestOptions,
) {
  const { job_id: param0, ...queryParams } = params
  return request<API.GenerationJobResponse>(`/generation-jobs/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 撤销并清理本人生成任务 DELETE /generation-jobs/${param0} */
export async function deleteGenerationJob(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteGenerationJobParams,
  options?: RequestOptions,
) {
  const { job_id: param0, ...queryParams } = params
  return request<any>(`/generation-jobs/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 请求取消本人生成任务 POST /generation-jobs/${param0}/cancel */
export async function cancelGenerationJob(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.cancelGenerationJobParams,
  options?: RequestOptions,
) {
  const { job_id: param0, ...queryParams } = params
  return request<any>(`/generation-jobs/${param0}/cancel`, {
    method: 'POST',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 获取本人已完成产物的短时读取凭据 GET /generation-jobs/${param0}/output-access */
export async function getGenerationOutputAccess(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getGenerationOutputAccessParams,
  options?: RequestOptions,
) {
  const { job_id: param0, ...queryParams } = params
  return request<API.GenerationOutputAccessResponse>(
    `/generation-jobs/${param0}/output-access`,
    {
      method: 'GET',
      params: { ...queryParams },
      ...(options || {}),
    },
  )
}
