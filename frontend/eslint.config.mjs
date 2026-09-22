import eslintConfigPrettier from 'eslint-config-prettier/flat'
import { defineConfig, globalIgnores } from 'eslint/config'
import nextVitals from 'eslint-config-next/core-web-vitals'
import nextTs from 'eslint-config-next/typescript'

const componentSystem = {
  group: [
    '@radix-ui/themes',
    '@radix-ui/themes/**',
    '@base-ui/*',
    'react-aria-components',
  ],
  message: 'Use the project shadcn/ui + Radix components in @/components/ui.',
}
const transport = {
  group: ['axios', 'axios/**'],
  message:
    'Use the generated API client; Axios belongs only in lib/api/request.ts.',
}

export default defineConfig([
  ...nextVitals,
  ...nextTs,
  eslintConfigPrettier,
  {
    files: ['src/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', { patterns: [componentSystem] }],
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/lib/api/request.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        { patterns: [componentSystem, transport] },
      ],
    },
  },
  {
    files: ['src/components/ui/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            componentSystem,
            transport,
            {
              regex: '^@/(?!components/ui/|lib/utils$)',
              message:
                'UI primitives may depend on sibling UI components and cn(), not application code.',
            },
          ],
        },
      ],
    },
  },
  globalIgnores([
    '.next/**',
    'out/**',
    'build/**',
    'next-env.d.ts',
    'src/api/**',
  ]),
])
