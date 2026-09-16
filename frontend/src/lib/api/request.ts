import axios, { type AxiosRequestConfig } from 'axios'

export type RequestOptions = Omit<
  AxiosRequestConfig,
  'baseURL' | 'url' | 'withCredentials'
> & {
  requestType?: 'form'
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
  const response = await client.request<T>({
    ...config,
    url,
  })

  return response.data
}
