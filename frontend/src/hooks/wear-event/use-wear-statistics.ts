'use client'

import { useQuery } from '@tanstack/react-query'
import { getWearStatistics } from '@/api/wearFeedback'

export function useWearStatistics(enabled: boolean, from: string, to: string) {
  return useQuery({
    queryKey: ['wear-statistics', from, to],
    queryFn: () => getWearStatistics({ from, to }),
    enabled: enabled && !!from && !!to,
    retry: false,
    refetchOnWindowFocus: false,
  })
}
