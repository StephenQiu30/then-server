import type { ReactNode } from 'react'
import { SiteFooter } from '@/layout/site-footer'
import { SiteHeader } from '@/layout/site-header'

export function SiteLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-svh flex-col">
      <SiteHeader />
      <main id="main-content" tabIndex={-1} className="w-full flex-1">
        {children}
      </main>
      <SiteFooter />
    </div>
  )
}
