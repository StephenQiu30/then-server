import type { ReactNode } from 'react'
import { SiteNav } from '@/components/layout/site-nav'

export function PageShell({
  children,
  home = false,
}: {
  children: ReactNode
  home?: boolean
}) {
  return (
    <>
      <SiteNav />
      <main
        id="main-content"
        tabIndex={-1}
        className={`mx-auto flex min-h-[calc(100svh-5rem)] w-full flex-col px-gutter min-[641px]:px-8 ${home ? 'max-w-none gap-8 py-0 pb-12 min-[641px]:py-0 min-[641px]:pb-16 min-[1069px]:px-12' : 'max-w-7xl gap-12 py-10 min-[641px]:py-12'}`}
      >
        {children}
      </main>
    </>
  )
}
