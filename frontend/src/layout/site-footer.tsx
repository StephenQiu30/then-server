import Link from 'next/link'
import { cn } from '@/lib/utils'
import { layoutContainerClassName } from '@/layout/container'

export function SiteFooter() {
  return (
    <footer className="bg-card">
      <div
        className={cn(
          layoutContainerClassName,
          'flex min-h-20 items-center justify-between gap-4 py-4 text-xs text-muted-foreground',
        )}
      >
        <Link href="/" className="inline-flex min-h-11 items-center">
          WEARLOG · 衣序
        </Link>
        <span>选衣 · 试穿 · 再决定</span>
      </div>
    </footer>
  )
}
