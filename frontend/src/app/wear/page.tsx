import type { Metadata } from 'next'
import { WearEventPanel } from '@/components/wear-event/wear-event-panel'

export const metadata: Metadata = {
  title: '实际穿着 · 于是',
  description: '主动记录和纠正实际穿着。',
}

export default function WearPage() {
  return <WearEventPanel />
}
