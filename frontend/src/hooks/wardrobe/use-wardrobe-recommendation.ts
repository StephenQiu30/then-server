'use client'

import { useCallback } from 'react'
import { recommendWardrobeOutfits } from '@/api/wardrobe'

export function useWardrobeRecommendation() {
  return useCallback(
    (request: API.WardrobeRecommendationRequest) =>
      recommendWardrobeOutfits(request),
    [],
  )
}
