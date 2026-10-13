import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { layoutContainerClassName } from '@/layout/container'

export function PageContainer({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        layoutContainerClassName,
        'flex flex-col gap-12 py-10 min-[641px]:py-12',
        className,
      )}
    >
      {children}
    </div>
  )
}
