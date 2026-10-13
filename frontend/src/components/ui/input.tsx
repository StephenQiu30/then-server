import * as React from 'react'
import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'min-h-11 w-full min-w-0 rounded-sm border-0 border-transparent bg-surface-muted bg-clip-padding min-[700px]:border-y-4 px-3 py-2 text-body transition-colors outline-none placeholder:text-muted-foreground disabled:pointer-events-none disabled:opacity-50 aria-invalid:ring-2 aria-invalid:ring-destructive',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
