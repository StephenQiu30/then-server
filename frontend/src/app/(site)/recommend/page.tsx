import type { Metadata } from 'next'
import { RecommendationView } from '@/components/recommendation/recommendation-view'
import { isSceneId, type SceneId } from '@/lib/recommendation/scenes'

export const metadata: Metadata = {
  title: '真实衣橱推荐 · 衣序',
  description: '确认场景与约束，从本人真实衣橱查看少量可解释的穿搭。',
}

export default async function RecommendPage({
  searchParams,
}: {
  searchParams: Promise<{ scene?: string }>
}) {
  const { scene } = await searchParams
  const selected: SceneId = scene && isSceneId(scene) ? scene : 'daily'
  return <RecommendationView key={selected} initialScene={selected} />
}
