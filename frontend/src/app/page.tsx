import { ArrowDownIcon } from 'lucide-react'
import { PageShell } from '@/components/layout/page-shell'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

export default function HomePage() {
  return (
    <PageShell>
      <header className="flex flex-col items-start gap-6">
        <Badge variant="secondary">Web 版准备中</Badge>
        <h1 className="page-title">于是 OOTD</h1>
        <p className="max-w-xl text-muted-foreground">
          让每天的穿搭，有迹可循。
        </p>
        <Button asChild>
          <a href="#about">
            了解于是
            <ArrowDownIcon data-icon="inline-end" />
          </a>
        </Button>
      </header>
      <section
        id="about"
        aria-labelledby="about-title"
        className="flex scroll-mt-8 flex-col gap-4"
      >
        <h2 id="about-title" className="font-display text-xl font-semibold">
          从你的衣橱，走进每一天。
        </h2>
        <p className="max-w-2xl text-muted-foreground">
          整理真实拥有的衣物，记录喜欢的搭配，回顾每一天的穿着。于是希望让下一次选择，多一点依据。
        </p>
        <p className="max-w-2xl text-muted-foreground">
          Web 版正在准备中，账户与穿搭功能尚未开放。
        </p>
      </section>
    </PageShell>
  )
}
