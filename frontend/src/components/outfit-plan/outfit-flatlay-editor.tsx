'use client'

import { useState } from 'react'
import Link from 'next/link'
import { PlusIcon, XIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { WardrobeItemImage } from '@/components/wardrobe/wardrobe-item-image'
import { categoryLabels, availabilityLabels } from '@/lib/wardrobe/form'
import { replaceGarment } from '@/lib/outfit-plan/selection'
import type { PlanForm } from '@/lib/outfit-plan/form'

const categories = Object.keys(
  categoryLabels,
) as API.WardrobeItemResponse['category'][]

export function OutfitFlatlayEditor({
  form,
  onChange,
  items,
  pending,
}: {
  form: PlanForm
  onChange: (form: PlanForm) => void
  items: API.WardrobeItemResponse[]
  pending: boolean
}) {
  const [category, setCategory] =
    useState<API.WardrobeItemResponse['category']>('top')
  const [search, setSearch] = useState('')
  const selected = items.filter((item) =>
    form.items.some((selection) => selection.item_id === item.id),
  )
  const filtered = items.filter(
    (item) =>
      item.category === category &&
      item.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()),
  )
  function choose(item: API.WardrobeItemResponse) {
    const next = replaceGarment(form.items, item, items)
    onChange({
      ...form,
      items: next,
      confirmedUnavailableIDs: form.confirmedUnavailableIDs.filter((id) =>
        next.some((entry) => entry.item_id === id),
      ),
    })
  }
  return (
    <div className="grid gap-6 min-[900px]:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
      <section
        aria-label="搭配预览"
        className="flex flex-col gap-4 rounded-xl bg-muted/30 p-4 min-[641px]:p-6"
      >
        <div>
          <h2 className="text-xl font-semibold">这一套，准备这样穿</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            已选 {form.items.length} 件 · 选择单品，替换对应类别
          </p>
        </div>
        <div className="grid grid-cols-2 items-start gap-3">
          {(
            [
              'top',
              'bottom',
              'shoes',
              'outerwear',
              'one_piece',
              'bag',
              'accessory',
            ] as const
          )
            .filter((slot) =>
              ['top', 'bottom'].includes(slot)
                ? !selected.some((item) => item.category === 'one_piece')
                : slot === 'shoes' ||
                  selected.some((item) => item.category === slot),
            )
            .map((slot) => {
              const slotItems = selected.filter(
                (item) => item.category === slot,
              )
              return (
                <div
                  key={slot}
                  className={slot === 'shoes' ? 'col-span-2' : ''}
                >
                  {slotItems.length ? (
                    slotItems.map((item) => (
                      <div key={item.id} className="relative">
                        <WardrobeItemImage
                          item={item}
                          className={
                            slot === 'shoes' ? 'aspect-[3/1]' : 'aspect-[4/5]'
                          }
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={`移除${item.name}`}
                          disabled={pending}
                          className="absolute top-1 right-1"
                          onClick={() =>
                            onChange({
                              ...form,
                              items: form.items.filter(
                                (entry) => entry.item_id !== item.id,
                              ),
                              confirmedUnavailableIDs:
                                form.confirmedUnavailableIDs.filter(
                                  (id) => id !== item.id,
                                ),
                            })
                          }
                        >
                          <XIcon />
                        </Button>
                        <p className="mt-2 text-center text-sm">{item.name}</p>
                      </div>
                    ))
                  ) : (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={pending}
                      onClick={() => setCategory(slot)}
                      className="h-32 w-full border-dashed"
                    >
                      <PlusIcon />
                      选择{categoryLabels[slot]}
                    </Button>
                  )}
                </div>
              )
            })}
        </div>
        <p className="text-sm text-muted-foreground">
          单品平铺预览。示例图沿用原素材，不生成或改绘衣物。
        </p>
      </section>
      <section
        aria-label="搭配单品选择"
        className="flex min-w-0 flex-col gap-4"
      >
        <div
          className="flex flex-wrap gap-2"
          role="group"
          aria-label="搭配分类"
        >
          {categories.map((value) => (
            <Button
              key={value}
              type="button"
              variant={category === value ? 'default' : 'outline'}
              aria-pressed={category === value}
              disabled={pending}
              onClick={() => setCategory(value)}
            >
              {categoryLabels[value]}
            </Button>
          ))}
        </div>
        <Input
          aria-label="搜索搭配单品"
          placeholder="搜索单品名称"
          value={search}
          disabled={pending}
          onChange={(event) => setSearch(event.target.value)}
        />
        {filtered.length === 0 ? (
          <div className="flex flex-col items-start gap-3 py-6 text-muted-foreground">
            <p>这个分类还没有匹配的衣物。</p>
            <Button asChild variant="outline">
              <Link href="/wardrobe">去整理衣橱</Link>
            </Button>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-3 min-[1200px]:grid-cols-3">
            {filtered.map((item) => {
              const active = form.items.some(
                (entry) => entry.item_id === item.id,
              )
              return (
                <button
                  key={item.id}
                  type="button"
                  disabled={
                    pending ||
                    (!active &&
                      form.items.length >= 20 &&
                      !selected.some(
                        (entry) => entry.category === item.category,
                      ))
                  }
                  aria-label={`选择${item.name}`}
                  aria-pressed={active}
                  onClick={() => choose(item)}
                  className={`flex min-w-0 flex-col gap-2 rounded-xl border p-2 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50 ${active ? 'border-primary bg-muted/40' : 'border-border hover:bg-muted/30'}`}
                >
                  <WardrobeItemImage item={item} className="w-full" />
                  <span className="px-1 text-sm font-medium break-words">
                    {item.name}
                  </span>
                  <span className="px-1 pb-1 text-xs text-muted-foreground">
                    {active ? '已选 · ' : ''}
                    {availabilityLabels[item.availability]}
                    {item.sample_image ? ' · 示例图' : ''}
                  </span>
                </button>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
