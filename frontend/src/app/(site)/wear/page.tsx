import type { Metadata } from 'next'
import { WearEventView } from '@/components/wear-event/wear-event-view'

export const metadata: Metadata = {
  title: '实际穿着 · 衣序',
  description: '主动记录和纠正实际穿着。',
}

export default async function WearPage({
  searchParams,
}: {
  searchParams: Promise<{ plan?: string }>
}) {
  const { plan } = await searchParams
  return <WearEventView initialPlanID={plan} />
}
