import { BrandLink } from '@/layout/brand-link'
import { cn } from '@/lib/utils'
import { layoutContainerClassName } from '@/layout/container'
import { DesktopNavigation, MobileNavigation } from '@/layout/site-navigation'

export function SiteHeader() {
  return (
    <header className="bg-card">
      <div
        className={cn(
          layoutContainerClassName,
          'flex min-h-14 min-[700px]:min-h-16 flex-wrap items-center justify-between gap-2 py-1.5 min-[860px]:gap-6',
        )}
      >
        <BrandLink />
        <DesktopNavigation />
        <MobileNavigation />
      </div>
    </header>
  )
}
