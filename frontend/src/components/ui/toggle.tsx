'use client'

import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { Toggle as TogglePrimitive } from 'radix-ui'

const toggleVariants = cva(
  "group/toggle inline-flex min-h-11 min-w-11 items-center justify-center gap-1 rounded-lg text-sm font-medium whitespace-nowrap transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 'bg-transparent data-[state=on]:bg-muted',
        solid:
          'rounded-sm border-y-0 border-transparent bg-selection bg-clip-padding min-[700px]:border-y-4 data-[state=on]:bg-primary data-[state=on]:text-primary-foreground',
        subtle:
          'rounded-sm bg-selection data-[state=on]:bg-primary data-[state=on]:text-primary-foreground',
        card: 'justify-start gap-3 rounded-sm bg-card p-2 text-left whitespace-normal data-[state=on]:bg-selection',
        tile: 'flex-col items-start gap-1 rounded-sm bg-transparent p-2 text-left text-xs font-normal whitespace-normal data-[state=on]:bg-selection',
        outline: 'bg-transparent hover:bg-muted',
        line: 'rounded-none bg-transparent text-muted-foreground hover:bg-transparent data-[state=on]:bg-transparent data-[state=on]:text-foreground data-[state=on]:underline data-[state=on]:underline-offset-4',
      },
      size: {
        default:
          'px-4 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3',
        sm: 'px-3 text-sm has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2',
        lg: 'px-5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

function Toggle({
  className,
  variant = 'default',
  size = 'default',
  ...props
}: React.ComponentProps<typeof TogglePrimitive.Root> &
  VariantProps<typeof toggleVariants>) {
  return (
    <TogglePrimitive.Root
      data-slot="toggle"
      className={cn(toggleVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Toggle, toggleVariants }
