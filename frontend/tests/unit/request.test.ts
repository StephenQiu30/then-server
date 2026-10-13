import { AxiosHeaders, type AxiosAdapter, type AxiosResponse } from 'axios'
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import request from '../../src/lib/api/request.ts'

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

it('omits cookies on signed storage PUT and exposes the immutable object version', async () => {
  let version: unknown
  await request('http://127.0.0.1:9000/raw-private/fixture', {
    method: 'PUT',
    withCredentials: false,
    onResponseHeaders: (headers) => {
      version = headers['x-amz-version-id']
    },
    adapter: async (config) => {
      assert.equal(config.withCredentials, false)
      assert.equal('onResponseHeaders' in config, false)
      return {
        config,
        data: '',
        headers: new AxiosHeaders({ 'x-amz-version-id': 'fixture-version' }),
        status: 200,
        statusText: 'OK',
      }
    },
  })
  assert.equal(version, 'fixture-version')
})
