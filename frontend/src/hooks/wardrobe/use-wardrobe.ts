'use client'

import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import {
  createWardrobeItem,
  deleteWardrobeItem,
  getWardrobeDeletionImpact,
  getWardrobeItem,
  listWardrobeItems,
  updateWardrobeItem,
} from '@/api/wardrobe'

const wardrobeKey = ['wardrobe', 'items'] as const

export function useWardrobeItems(enabled: boolean) {
  return useInfiniteQuery({
    queryKey: wardrobeKey,
    enabled,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      listWardrobeItems({ limit: 30, after_id: pageParam }),
    getNextPageParam: (page) => page.next_after_id ?? undefined,
    retry: false,
    refetchOnWindowFocus: false,
  })
}

export function useWardrobeActions() {
  const queryClient = useQueryClient()
  return useMemo(
    () => ({
      async create(body: API.CreateWardrobeItemRequest) {
        const item = await createWardrobeItem(body)
        await queryClient.invalidateQueries({ queryKey: wardrobeKey })
        return item
      },
      async update(id: string, body: API.UpdateWardrobeItemRequest) {
        const item = await updateWardrobeItem({ item_id: id }, body)
        await queryClient.invalidateQueries({ queryKey: wardrobeKey })
        return item
      },
      get: (id: string) => getWardrobeItem({ item_id: id }),
      impact: (id: string) => getWardrobeDeletionImpact({ item_id: id }),
      async remove(
        id: string,
        revision: number,
        impact: string,
        policy: 'redact_snapshots' | 'delete_affected_history',
      ) {
        await deleteWardrobeItem({
          item_id: id,
          expected_revision: revision,
          expected_impact: impact,
          history_policy: policy,
        })
        await queryClient.invalidateQueries({ queryKey: wardrobeKey })
      },
    }),
    [queryClient],
  )
}
