import axios, { type AxiosRequestConfig } from 'axios'

export type RequestOptions = Omit<AxiosRequestConfig, 'baseURL' | 'url'> & {
  requestType?: 'form'
  onResponseHeaders?: (headers: Record<string, unknown>) => void
}

const client = axios.create({
  headers: {
    Accept: 'application/json',
  },
  withCredentials: true,
})

export default async function request<T>(
  url: string,
  options: RequestOptions = {},
): Promise<T> {
  const config = { ...options }
  delete config.requestType
  delete config.onResponseHeaders
  const response = await client.request<T>({
    ...config,
    url,
  })

  options.onResponseHeaders?.(response.headers as Record<string, unknown>)
  return response.data
}
