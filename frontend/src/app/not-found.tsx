import Link from 'next/link'
import { PageShell } from '@/components/layout/page-shell'
import { Button } from '@/components/ui/button'

export default function NotFound() {
  return (
    <PageShell>
      <div className="flex flex-col items-start gap-6">
        <h1 className="page-title">没有找到这个页面</h1>
        <p className="text-muted-foreground">
          链接可能已失效，你可以返回首页继续浏览。
        </p>
        <Button asChild>
          <Link href="/">返回首页</Link>
        </Button>
      </div>
    </PageShell>
  )
}
