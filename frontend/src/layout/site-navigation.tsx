'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { MenuIcon, XIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  NavigationMenu,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
} from '@/components/ui/navigation-menu'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { headerLinks, isSiteLinkCurrent } from '@/layout/site-links'

export function DesktopNavigation() {
  const pathname = usePathname()

  return (
    <NavigationMenu
      aria-label="主要导航"
      viewport={false}
      className="hidden min-[860px]:flex"
    >
      <NavigationMenuList className="gap-6">
        {headerLinks.map(({ href, label }) => {
          const active = isSiteLinkCurrent(href, pathname)
          return (
            <NavigationMenuItem key={href}>
              <NavigationMenuLink asChild active={active}>
                <Link href={href} aria-current={active ? 'page' : undefined}>
                  {label}
                </Link>
              </NavigationMenuLink>
            </NavigationMenuItem>
          )
        })}
      </NavigationMenuList>
    </NavigationMenu>
  )
}

export function MobileNavigation() {
  const pathname = usePathname()

  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label="打开导航菜单"
          className="min-[860px]:hidden"
        >
          <MenuIcon aria-hidden="true" data-icon="inline-start" />
        </Button>
      </SheetTrigger>
      <SheetContent side="right" showCloseButton={false}>
        <SheetHeader className="flex-row items-start justify-between gap-4 p-6">
          <div>
            <SheetTitle>衣序</SheetTitle>
            <SheetDescription className="sr-only">
              选择要前往的页面
            </SheetDescription>
          </div>
          <SheetClose asChild>
            <Button variant="ghost" size="icon" aria-label="关闭导航菜单">
              <XIcon aria-hidden="true" data-icon="inline-start" />
            </Button>
          </SheetClose>
        </SheetHeader>
        <nav aria-label="移动端导航" className="px-4 pb-6">
          <ul className="flex flex-col gap-1">
            {headerLinks.map(({ href, label }) => {
              const active = isSiteLinkCurrent(href, pathname)
              return (
                <li key={href}>
                  <SheetClose asChild>
                    <Link
                      href={href}
                      aria-current={active ? 'page' : undefined}
                      className={cn(
                        'flex min-h-12 items-center rounded-sm px-4 text-base transition-colors',
                        active
                          ? 'font-semibold text-foreground underline underline-offset-4'
                          : 'text-muted-foreground hover:text-foreground',
                      )}
                    >
                      {label}
                    </Link>
                  </SheetClose>
                </li>
              )
            })}
          </ul>
        </nav>
      </SheetContent>
    </Sheet>
  )
}
