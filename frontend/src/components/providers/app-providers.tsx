'use client'

import { Theme } from '@radix-ui/themes'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'

export function AppProviders({ children }: { children: ReactNode }) {
  const [queryClient] = useState(() => new QueryClient())

  return (
    <Theme accentColor="teal" grayColor="sand" radius="large">
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </Theme>
  )
}
