'use client'

import Link from 'next/link'
import { useTransition } from 'react'
import { PageShell } from '@/components/layout/page-shell'
import { Button } from '@/components/ui/button'

export default function ErrorPage({ retry }: { retry: () => void }) {
  const [isPending, startTransition] = useTransition()
  return (
    <PageShell>
      <div className="flex flex-col items-start gap-6" aria-busy={isPending}>
        <h1 className="page-title">页面暂时无法显示</h1>
        <p className="text-muted-foreground">
          请重试。如果仍无法恢复，可以先返回首页。
        </p>
        <div className="flex flex-wrap gap-3">
          <Button disabled={isPending} onClick={() => startTransition(retry)}>
            {isPending ? '正在重试…' : '重试'}
          </Button>
          <Button variant="secondary" asChild>
            <Link href="/">返回首页</Link>
          </Button>
        </div>
      </div>
    </PageShell>
  )
}
