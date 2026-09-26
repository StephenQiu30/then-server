'use client'

import { useState } from 'react'
import Image from 'next/image'
import Link from 'next/link'
import {
  ArrowRightIcon,
  BriefcaseBusinessIcon,
  CoffeeIcon,
  HeartIcon,
  HouseIcon,
  LuggageIcon,
  UsersRoundIcon,
} from 'lucide-react'
import { PageShell } from '@/components/layout/page-shell'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
} from '@/components/ui/card'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  scenes,
  type SceneId,
  isSceneId,
} from '@/components/recommendation/scenes'

const sceneIcons = {
  daily: CoffeeIcon,
  commute: BriefcaseBusinessIcon,
  friends: UsersRoundIcon,
  date: HeartIcon,
  travel: LuggageIcon,
  relax: HouseIcon,
}

const inspiration = [
  {
    scene: 'daily',
    title: '温柔舒适的日常',
    description: '让熟悉的单品陪你出门，在日常里留一点轻松。',
    image: '/images/look-daily.png',
    alt: '燕麦色针织衫与米白长裤的示例穿搭',
  },
  {
    scene: 'commute',
    title: '简约利落的通勤',
    description: '从容又利落，让忙碌的一天穿得更自在。',
    image: '/images/look-work.png',
    alt: '浅蓝衬衫与深色长裤的示例穿搭',
  },
  {
    scene: 'date',
    title: '轻松有型的相聚',
    description: '给见面多一点心意，也保留自己的节奏。',
    image: '/images/look-evening.png',
    alt: '黑色外套与条纹内搭的示例穿搭',
  },
] as const

export function HomeExperience() {
  const [scene, setScene] = useState<SceneId>('daily')
  return (
    <PageShell home>
      <section
        aria-labelledby="home-title"
        className="grid items-center gap-8 min-[900px]:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)] min-[900px]:gap-10"
      >
        <div className="flex flex-col items-start gap-6 py-10">
          <div className="flex flex-col gap-4">
            <h1
              id="home-title"
              className="home-title max-w-[10ch] font-semibold"
            >
              今天想怎么穿？
            </h1>
            <p className="max-w-md text-lg leading-relaxed text-muted-foreground">
              从一个真实的生活场景开始，在你的衣橱里找到适合今天的搭配。
            </p>
          </div>
          <div className="flex w-full flex-col gap-3">
            <p id="scene-label" className="text-sm font-medium">
              选择今天的场景
            </p>
            <ToggleGroup
              type="single"
              aria-labelledby="scene-label"
              value={scene}
              onValueChange={(value) => {
                if (value && isSceneId(value)) setScene(value)
              }}
              className="flex w-full flex-wrap justify-start gap-2"
            >
              {scenes.map((option) => {
                const Icon = sceneIcons[option.id]
                return (
                  <ToggleGroupItem
                    key={option.id}
                    value={option.id}
                    variant="scene"
                  >
                    <Icon data-icon="inline-start" />
                    {option.label}
                  </ToggleGroupItem>
                )
              })}
            </ToggleGroup>
          </div>
          <Button
            asChild
            size="hero"
            className="w-full min-[420px]:w-auto min-[900px]:min-w-80"
          >
            <Link href={{ pathname: '/recommend', query: { scene } }}>
              为这个场景推荐穿搭 <ArrowRightIcon data-icon="inline-end" />
            </Link>
          </Button>
          <p className="text-sm text-muted-foreground">
            推荐只使用你已确认、当前可用的真实衣物。
          </p>
        </div>
        <figure className="relative overflow-hidden rounded-lg bg-muted min-[1069px]:-mr-12 min-[1069px]:rounded-r-none">
          <Image
            src="/images/scene-hero.png"
            alt="穿燕麦色针织衫与米色长裤的示例人物照片"
            width={1603}
            height={981}
            priority
            sizes="(max-width: 900px) 100vw, 55vw"
            className="aspect-[1.55] w-full object-cover min-[900px]:aspect-[1.88]"
          />
          <figcaption className="absolute right-4 bottom-4 rounded-md bg-card/95 px-3 py-2 text-xs text-muted-foreground">
            风格示例 · 非你的衣橱
          </figcaption>
        </figure>
      </section>

      <section
        aria-labelledby="inspiration-title"
        className="flex flex-col gap-5"
      >
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="flex flex-col gap-1">
            <h2
              id="inspiration-title"
              className="text-2xl font-semibold tracking-tight"
            >
              穿搭灵感
            </h2>
            <p className="text-sm text-muted-foreground">
              这里展示的是风格示例；你的推荐会使用本人衣橱。
            </p>
          </div>
          <Button asChild variant="link">
            <Link href="/wardrobe">
              先整理我的衣橱 <ArrowRightIcon data-icon="inline-end" />
            </Link>
          </Button>
        </div>
        <div className="grid gap-4 min-[680px]:grid-cols-3">
          {inspiration.map((look) => (
            <Card key={look.scene} className="group flex-row gap-0 py-0">
              <div className="w-[42%] shrink-0 overflow-hidden bg-muted">
                <Image
                  src={look.image}
                  alt={look.alt}
                  width={680}
                  height={850}
                  sizes="(max-width: 680px) 100vw, 33vw"
                  className="h-full min-h-56 w-full object-cover object-top transition-transform duration-300 group-hover:scale-[1.025] motion-reduce:transform-none"
                />
              </div>
              <div className="flex min-w-0 flex-1 flex-col justify-between py-5">
                <CardHeader className="gap-3 px-4">
                  <Badge variant="secondary" className="w-fit">
                    示例灵感
                  </Badge>
                  <CardTitle>
                    <h3>{look.title}</h3>
                  </CardTitle>
                  <CardDescription>{look.description}</CardDescription>
                </CardHeader>
                <Button asChild variant="link" className="w-fit px-4">
                  <Link
                    href={{
                      pathname: '/recommend',
                      query: { scene: look.scene },
                    }}
                  >
                    去此场景推荐 <ArrowRightIcon data-icon="inline-end" />
                  </Link>
                </Button>
              </div>
            </Card>
          ))}
        </div>
      </section>
    </PageShell>
  )
}
