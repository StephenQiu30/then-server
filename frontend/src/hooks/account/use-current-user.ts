'use client'

import { useQuery } from '@tanstack/react-query'
import { getCurrentUser } from '@/api/account'

export const currentUserKey = ['account', 'current-user'] as const

export function useCurrentUser() {
  return useQuery({
    queryKey: currentUserKey,
    queryFn: () => getCurrentUser(),
    retry: false,
    refetchOnWindowFocus: false,
  })
}
