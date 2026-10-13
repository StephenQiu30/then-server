'use client'

import { useRef, useState, type ReactNode } from 'react'
import Image from 'next/image'
import Link from 'next/link'
import { ShirtIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Toggle } from '@/components/ui/toggle'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { GarmentOptions } from '@/components/wardrobe/garment-options'
import type { PreservedGarment, GarmentOption } from '@/lib/wardrobe/selection'

export function GarmentPicker({
  items,
  selected,
  preserved,
  onSelected,
  action,
  secondaryAction,
  helperText,
}: {
  items: readonly GarmentOption[]
  selected: GarmentOption
  preserved: readonly PreservedGarment[]
  onSelected: (next: GarmentOption) => void
  action: ReactNode
  secondaryAction: ReactNode
  helperText: string
}) {
  const [choosing, setChoosing] = useState(false)
  const [query, setQuery] = useState('')
  const [showAll, setShowAll] = useState(false)
  const category = showAll ? 'all' : selected.category
  const returnFocus = useRef<HTMLButtonElement | null>(null)
  const part = selected.category === 'top' ? '上衣' : '下装'

  function changeCategory(value: string) {
    if (!value) return
    setShowAll(value === 'all')
    const next = items.find((item) => item.category === value)
    if (next && selected.category !== value) onSelected(next)
  }

  return (
    <Sheet
      open={choosing}
      onOpenChange={(open) => {
        setChoosing(open)
        if (!open) setShowAll(false)
      }}
    >
      <aside
        aria-labelledby="selection-title"
        className="flex min-w-0 flex-col min-[700px]:gap-2 min-[700px]:p-5"
      >
        <div className="hidden flex-col gap-2 min-[700px]:flex">
          <h2
            id="selection-title"
            className="text-lg leading-[26px] font-medium"
          >
            换一件，新感觉。
          </h2>
          <p className="text-xs leading-[18px] text-muted-foreground">
            从已有的衣服开始 · 示例衣橱
          </p>
          <GarmentOptions
            items={items}
            selected={selected}
            category={category}
            query={query}
            onSelected={onSelected}
            onCategoryChange={changeCategory}
            onQueryChange={setQuery}
          />
          <p className="text-xs leading-[18px] text-muted-foreground">
            其余衣物 · 保持原样
          </p>
          <div className="grid grid-cols-2 gap-4">
            {preserved.map((item) => (
              <figure
                key={item.id}
                className="flex min-h-45 min-w-0 flex-col gap-1 p-2"
              >
                <Image
                  src={item.image}
                  width={108}
                  height={108}
                  alt={item.name}
                  className="h-27 w-full object-contain"
                />
                <figcaption className="text-xs leading-[18px] text-muted-foreground">
                  {item.name}
                </figcaption>
                <p className="text-xs leading-[18px] text-muted-foreground">
                  保留 · {item.categoryLabel}
                </p>
              </figure>
            ))}
          </div>
          <p className="text-xs leading-[18px] text-muted-foreground">
            这次只换{part}，其余保持原样。
          </p>
        </div>
        <div className="bg-surface-muted px-5 pb-5 min-[700px]:hidden">
          <SheetTrigger asChild>
            <Toggle
              variant="card"
              pressed
              className="w-full"
              aria-label={'已选择' + selected.name + '，更换衣服'}
              onClick={(event) => {
                returnFocus.current = event.currentTarget
                setShowAll(true)
              }}
            >
              <Image
                src={selected.image}
                width={64}
                height={64}
                alt=""
                className="size-16 shrink-0 object-contain"
              />
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span>{selected.name}</span>
                <span className="text-xs font-normal text-muted-foreground">
                  本次只换{part}，其余保持原样。
                </span>
              </span>
            </Toggle>
          </SheetTrigger>
        </div>
        <div className="flex gap-2 px-5 py-5 min-[700px]:flex-col min-[700px]:gap-0 min-[700px]:p-0">
          <SheetTrigger asChild>
            <Button
              variant="surface"
              className="min-[700px]:hidden"
              onClick={(event) => {
                returnFocus.current = event.currentTarget
                setShowAll(true)
              }}
            >
              <ShirtIcon data-icon="inline-start" aria-hidden="true" />
              换一件
            </Button>
          </SheetTrigger>
          {action}
          <div className="hidden w-full min-[700px]:block">
            {secondaryAction}
          </div>
        </div>
        <div className="flex flex-col gap-1 px-5 pb-5 min-[700px]:px-0">
          <p className="text-xs leading-[18px] text-muted-foreground">
            {helperText}
          </p>
          <div className="min-[700px]:hidden">{secondaryAction}</div>
        </div>

        <SheetContent
          side="bottom"
          showCloseButton={false}
          className="max-h-svh overflow-y-auto data-[side=bottom]:h-svh"
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            returnFocus.current?.focus()
          }}
        >
          <SheetHeader className="px-5 pt-6 pb-0">
            <SheetTitle>这次，想换哪一件？</SheetTitle>
            <SheetDescription>
              每次只选一件，重新选择会替换当前单品。
            </SheetDescription>
          </SheetHeader>
          <div className="flex flex-1 flex-col gap-4 px-5">
            <GarmentOptions
              items={items}
              selected={selected}
              category={category}
              query={query}
              onSelected={onSelected}
              onCategoryChange={changeCategory}
              onQueryChange={setQuery}
              showAllCategory
            />
          </div>
          <div className="flex gap-2 p-5">
            <SheetClose asChild>
              <Button variant="surface">返回</Button>
            </SheetClose>
            <SheetClose asChild>
              <Button className="flex-1">使用这件衣服</Button>
            </SheetClose>
          </div>
          <Button asChild variant="ghost" className="mx-5 mb-4">
            <Link href="/wardrobe">管理我的真实衣橱</Link>
          </Button>
        </SheetContent>
      </aside>
    </Sheet>
  )
}
