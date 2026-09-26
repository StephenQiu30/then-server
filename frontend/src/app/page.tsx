import type { Metadata } from 'next'
import { HomeExperience } from '@/components/recommendation/home-experience'

export const metadata: Metadata = {
  title: '今天想怎么穿？· 于是 OOTD',
  description: '从真实衣橱出发，按场景选择少量可解释的穿搭。',
}

export default function HomePage() {
  return <HomeExperience />
}
