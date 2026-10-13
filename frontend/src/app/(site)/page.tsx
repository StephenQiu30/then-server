import type { Metadata } from 'next'
import { FittingRoomView } from '@/components/fitting-room/fitting-room-view'

export const metadata: Metadata = {
  title: '试衣间 · 衣序',
  description: '挑选衣服，看看上身效果，对比之后再决定。',
}

export default function HomePage() {
  return <FittingRoomView />
}
