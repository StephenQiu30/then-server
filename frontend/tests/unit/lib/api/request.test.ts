import { AxiosHeaders, type AxiosAdapter, type AxiosResponse } from 'axios'
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import request from '../../../../src/lib/api/request.ts'

describe('request', () => {
  it('uses the shared Axios defaults and returns only response data', async () => {
    let observedURL = ''
    let observedCredentials = false
    const adapter: AxiosAdapter = async (config) => {
      observedURL = config.url ?? ''
      observedCredentials = config.withCredentials === true
      assert.equal('requestType' in config, false)

      return {
        config,
        data: { status: 'live' },
        headers: new AxiosHeaders(),
        status: 200,
        statusText: 'OK',
      } satisfies AxiosResponse<{ status: string }>
    }

    const response = await request<{ status: string }>('/health/live', {
      adapter,
      method: 'GET',
      requestType: 'form',
    })

    assert.deepEqual(response, { status: 'live' })
    assert.equal(observedURL, '/health/live')
    assert.equal(observedCredentials, true)
  })
})
