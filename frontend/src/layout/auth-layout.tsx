import type { ReactNode } from 'react'
import { BrandLink } from '@/layout/brand-link'
import Link from 'next/link'
import { Button } from '@/components/ui/button'

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <main
      id="main-content"
      tabIndex={-1}
      className="flex min-h-svh flex-col items-center justify-center gap-6 bg-muted px-4 py-8 min-[480px]:px-6 min-[480px]:py-10"
    >
      <div className="flex w-full max-w-sm flex-col gap-6">
        <div className="self-center">
          <BrandLink />
        </div>
        {children}
        <Button variant="link" asChild className="self-center">
          <Link href="/">返回首页</Link>
        </Button>
      </div>
    </main>
  )
}
