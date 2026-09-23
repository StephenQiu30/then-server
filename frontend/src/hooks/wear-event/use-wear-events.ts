'use client'

import { useMemo } from 'react'
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import {
  createWearEvent,
  deleteWearEvent,
  getWearEvent,
  listWearEvents,
  updateWearEvent,
} from '@/api/wearEvents'

const wearKey = ['wear-events'] as const

export function useWearEvents(enabled: boolean) {
  return useInfiniteQuery({
    queryKey: wearKey,
    enabled,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      listWearEvents({ limit: 20, after_id: pageParam }),
    getNextPageParam: (page) => page.next_after_id ?? undefined,
    retry: false,
    refetchOnWindowFocus: false,
  })
}

export function useWearEventActions() {
  const queryClient = useQueryClient()
  return useMemo(() => {
    async function changed<T>(operation: () => Promise<T>): Promise<T> {
      const result = await operation()
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: wearKey }),
        queryClient.invalidateQueries({ queryKey: ['outfit-plans'] }),
        queryClient.invalidateQueries({ queryKey: ['wardrobe', 'items'] }),
      ])
      return result
    }
    return {
      create: (body: API.CreateWearEventRequest) =>
        changed(() => createWearEvent(body)),
      update: (id: string, body: API.UpdateWearEventRequest) =>
        changed(() => updateWearEvent({ wear_event_id: id }, body)),
      get: (id: string) => getWearEvent({ wear_event_id: id }),
      remove: (id: string, revision: number) =>
        changed(() =>
          deleteWearEvent({ wear_event_id: id, expected_revision: revision }),
        ),
    }
  }, [queryClient])
}
