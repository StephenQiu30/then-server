import type { ReactNode } from 'react'
import { PageShell } from '@/components/layout/page-shell'
import { Badge } from '@/components/ui/badge'

export function AccountShell({
  title,
  description,
  children,
}: {
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <PageShell>
      <div className="flex max-w-5xl flex-col gap-8 min-[800px]:grid min-[800px]:grid-cols-2 min-[800px]:items-start min-[800px]:gap-16">
        <header className="flex flex-col items-start gap-5">
          <Badge variant="secondary">账户</Badge>
          <h1 className="page-title">{title}</h1>
          <p className="max-w-md text-muted-foreground">{description}</p>
        </header>
        <div className="w-full min-w-0">{children}</div>
      </div>
    </PageShell>
  )
}
