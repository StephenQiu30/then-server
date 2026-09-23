'use client'

import { useMemo } from 'react'
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import {
  cancelOutfitPlan,
  createOutfitPlan,
  deleteOutfitPlan,
  getOutfitPlan,
  listOutfitPlans,
  markOutfitPlanNotWorn,
  restoreOutfitPlan,
  updateOutfitPlan,
} from '@/api/outfitPlans'

const planKey = ['outfit-plans'] as const

export function useOutfitPlans(enabled: boolean) {
  return useInfiniteQuery({
    queryKey: planKey,
    enabled,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      listOutfitPlans({ limit: 20, after_id: pageParam }),
    getNextPageParam: (page) => page.next_after_id ?? undefined,
    retry: false,
    refetchOnWindowFocus: false,
  })
}

export function useOutfitPlanActions() {
  const queryClient = useQueryClient()
  return useMemo(() => {
    async function changed<T>(operation: () => Promise<T>): Promise<T> {
      const result = await operation()
      await queryClient.invalidateQueries({ queryKey: planKey })
      return result
    }

    return {
      create: (body: API.CreateOutfitPlanRequest) =>
        changed(() => createOutfitPlan(body)),
      update: (id: string, body: API.UpdateOutfitPlanRequest) =>
        changed(() => updateOutfitPlan({ plan_id: id }, body)),
      get: (id: string) => getOutfitPlan({ plan_id: id }),
      cancel: (id: string, revision: number) =>
        changed(() =>
          cancelOutfitPlan({ plan_id: id }, { expected_revision: revision }),
        ),
      markNotWorn: (id: string, revision: number) =>
        changed(() =>
          markOutfitPlanNotWorn(
            { plan_id: id },
            { expected_revision: revision },
          ),
        ),
      restore: (id: string, revision: number) =>
        changed(() =>
          restoreOutfitPlan({ plan_id: id }, { expected_revision: revision }),
        ),
      remove: (id: string, revision: number) =>
        changed(() =>
          deleteOutfitPlan({ plan_id: id, expected_revision: revision }),
        ),
    }
  }, [queryClient])
}
