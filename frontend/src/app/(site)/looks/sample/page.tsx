import type { Metadata } from 'next'
import { LookDetailView } from '@/components/look/look-detail-view'

export const metadata: Metadata = { title: '搭配详情 · 衣序' }
export default function LookPage() {
  return <LookDetailView />
}
