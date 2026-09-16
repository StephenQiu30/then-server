// @ts-ignore
/* eslint-disable */
import request, { type RequestOptions } from '../request'

/** 检查 API 进程存活 仅供受限运维访问，不查询数据库，不代表云业务已启用。 GET /health/live */
export async function getLiveness(options?: RequestOptions) {
  return request<API.LivenessResponse>('/health/live', {
    method: 'GET',
    ...(options || {}),
  })
}

/** 检查 API 接纳就绪状态 在同一有界上下文中探测 PostgreSQL 与认证 Redis；退出或任一依赖故障时返回 503，不输出连接详情。 GET /health/ready */
export async function getReadiness(options?: RequestOptions) {
  return request<API.ReadinessResponse>('/health/ready', {
    method: 'GET',
    ...(options || {}),
  })
}
