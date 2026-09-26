'use client'

import Link from 'next/link'
import Image from 'next/image'
import { usePathname } from 'next/navigation'
import { UserRoundIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'

const links = [
  { href: '/', label: '今日推荐' },
  { href: '/wardrobe', label: '我的衣橱' },
  { href: '/plans', label: '穿搭计划' },
  { href: '/wear', label: '实际穿着' },
] as const

export function SiteNav() {
  const pathname = usePathname()
  return (
    <header className="border-b border-border bg-card">
      <div className="mx-auto flex w-full max-w-none flex-wrap items-center justify-between gap-x-8 gap-y-2 px-gutter py-4 min-[641px]:px-8 min-[1069px]:px-12">
        <Link
          href="/"
          className="inline-flex min-h-11 items-center gap-2 whitespace-nowrap"
          aria-label="穿见，返回首页"
        >
          <Image
            src="/brand/chuanjian-icon.png"
            alt=""
            width={36}
            height={36}
            className="size-9 shrink-0"
            priority
          />
          <span className="text-xl font-semibold tracking-tight">穿见</span>
          <span className="hidden border-l border-border pl-2 text-[10px] font-medium tracking-[0.18em] text-muted-foreground min-[641px]:inline">
            CHUANJIAN
          </span>
        </Link>
        <nav
          aria-label="主要导航"
          className="order-3 w-full min-[720px]:order-2 min-[720px]:w-auto"
        >
          <ul className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm min-[641px]:gap-x-6">
            {links.map(({ href, label }) => {
              const active =
                href === '/'
                  ? pathname === '/' || pathname === '/recommend'
                  : pathname === href
              return (
                <li key={href}>
                  <Link
                    href={href}
                    aria-current={active ? 'page' : undefined}
                    className="inline-flex min-h-11 items-center border-b-2 border-transparent text-muted-foreground transition-colors hover:text-foreground aria-[current=page]:border-primary aria-[current=page]:font-medium aria-[current=page]:text-foreground"
                  >
                    {label}
                  </Link>
                </li>
              )
            })}
          </ul>
        </nav>
        <Button
          asChild
          variant="ghost"
          size="icon"
          className="order-2 min-[720px]:order-3"
        >
          <Link href="/account" aria-label="账户">
            <UserRoundIcon aria-hidden="true" />
          </Link>
        </Button>
      </div>
    </header>
  )
}
