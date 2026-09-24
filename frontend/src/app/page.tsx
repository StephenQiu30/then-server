import { ArrowDownIcon } from 'lucide-react'
import Link from 'next/link'
import { PageShell } from '@/components/layout/page-shell'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

export default function HomePage() {
  return (
    <PageShell>
      <header className="home-hero flex flex-col items-start gap-6 rounded-lg border border-border px-6 py-12 min-[641px]:px-12 min-[641px]:py-16">
        <Badge variant="secondary">于是 OOTD</Badge>
        <h1 className="home-title font-semibold">让每天的穿搭，有迹可循。</h1>
        <p className="max-w-xl text-muted-foreground">
          整理你的衣橱，计划想穿的搭配，记录真实的一天。
        </p>
        <div className="flex flex-wrap gap-3">
          <Button asChild>
            <Link href="/register">创建账户</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/login">登录</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/wardrobe">查看衣橱</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/plans">查看穿搭计划</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/wear">查看实际穿着</Link>
          </Button>
          <Button asChild variant="ghost">
            <a href="#about">
              了解于是
              <ArrowDownIcon data-icon="inline-end" />
            </a>
          </Button>
        </div>
      </header>
      <section
        id="about"
        aria-labelledby="about-title"
        className="flex scroll-mt-8 flex-col gap-4"
      >
        <h2 id="about-title" className="text-xl font-semibold tracking-tight">
          从你的衣橱，走进每一天。
        </h2>
        <p className="max-w-2xl text-muted-foreground">
          整理真实拥有的衣物，记录喜欢的搭配，回顾每一天的穿着。于是希望让下一次选择，多一点依据。
        </p>
        <p className="max-w-2xl text-muted-foreground">
          账户、无图衣橱、计划和实际记录页面可用于本地验证。离线
          Look、推荐与反馈页面仍在准备中。
        </p>
      </section>
    </PageShell>
  )
}
