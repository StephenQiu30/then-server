import type { Metadata } from 'next'
import type { ReactNode } from 'react'
import { AppProviders } from '@/components/providers/app-providers'
import '@radix-ui/themes/styles.css'
import './globals.css'

export const metadata: Metadata = {
  title: '于是 OOTD',
  description: '基于真实衣橱的穿搭决策助手',
}

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <AppProviders>{children}</AppProviders>
      </body>
    </html>
  )
}
