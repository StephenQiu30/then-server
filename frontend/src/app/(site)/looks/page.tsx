import type { Metadata } from 'next'
import { LookCollectionView } from '@/components/look/look-collection-view'

export const metadata: Metadata = { title: '搭配收藏 · 衣序' }
export default function LooksPage() {
  return <LookCollectionView />
}
