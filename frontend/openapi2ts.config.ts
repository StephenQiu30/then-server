import type { GenerateServiceProps } from '@umijs/openapi'

const config = {
  schemaPath:
    process.env.THEN_OPENAPI_SCHEMA ?? 'http://127.0.0.1:8080/openapi.json',
  serversPath: './src/lib/api',
  projectName: 'generated',
  namespace: 'API',
  requestImportStatement:
    "import request, { type RequestOptions } from '../request';",
  requestOptionsType: 'RequestOptions',
} satisfies GenerateServiceProps

export default config
