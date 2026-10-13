import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { Slot } from 'radix-ui'

const buttonVariants = cva(
  "group/button inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center gap-2 rounded-sm border-0 border-transparent bg-clip-padding min-[700px]:border-y-4 text-sm font-medium whitespace-normal text-center transition-colors select-none disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground',
        outline: 'bg-selection text-foreground hover:bg-muted',
        'outline-subtle': 'bg-selection text-foreground hover:bg-muted',
        surface: 'bg-selection text-foreground hover:bg-muted',
        secondary: 'bg-secondary text-secondary-foreground',
        ghost:
          'bg-transparent text-foreground hover:bg-muted focus-visible:bg-muted',
        destructive: 'bg-destructive text-destructive-foreground',
        link: 'text-link underline underline-offset-4',
      },
      size: {
        default: 'px-4 py-1',
        xs: 'px-3 py-2 text-caption',
        sm: 'px-4 py-2 text-caption',
        lg: 'px-6 py-3 text-base',
        xl: 'min-h-14 px-8 py-4 text-base',
        icon: 'size-11',
        'icon-xs': 'size-11',
        'icon-sm': 'size-11',
        'icon-lg': 'size-12',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

function Button({
  className,
  variant = 'default',
  size = 'default',
  asChild = false,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : 'button'

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
