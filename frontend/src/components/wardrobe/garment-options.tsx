'use client'

import { useId } from 'react'
import Image from 'next/image'
import { CheckIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import type { GarmentOption } from '@/lib/wardrobe/selection'

export function GarmentOptions({
  items,
  selected,
  category,
  query,
  showAllCategory = false,
  onSelected,
  onCategoryChange,
  onQueryChange,
}: {
  items: readonly GarmentOption[]
  selected: GarmentOption
  category: string
  query: string
  showAllCategory?: boolean
  onSelected: (item: GarmentOption) => void
  onCategoryChange: (category: string) => void
  onQueryChange: (query: string) => void
}) {
  const id = useId()
  const visibleItems = items.filter(
    (item) =>
      (category === 'all' || item.category === category) &&
      (item.name + item.color).includes(query.trim()),
  )
  return (
    <>
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor={id} className="sr-only">
            搜索示例衣橱
          </FieldLabel>
          <Input
            id={id}
            type="search"
            placeholder="搜索我的衣橱"
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
          />
        </Field>
      </FieldGroup>
      <ToggleGroup
        type="single"
        variant="line"
        value={category}
        spacing={2}
        className="w-full"
        aria-label="衣物类别"
        onValueChange={onCategoryChange}
      >
        {showAllCategory && (
          <ToggleGroupItem value="all" className="flex-1">
            全部
          </ToggleGroupItem>
        )}
        <ToggleGroupItem value="top" className="flex-1">
          上装
        </ToggleGroupItem>
        <ToggleGroupItem value="bottom" className="flex-1">
          下装
        </ToggleGroupItem>
      </ToggleGroup>
      {visibleItems.length ? (
        <ToggleGroup
          type="single"
          variant="tile"
          value={selected.id}
          spacing={4}
          className="grid w-full grid-cols-2 items-start"
          aria-label="选择一件示例衣物"
          onValueChange={(value) => {
            const next = items.find((item) => item.id === value)
            if (next) onSelected(next)
          }}
        >
          {visibleItems.map((item) => (
            <ToggleGroupItem
              key={item.id}
              value={item.id}
              className="min-h-45 w-full"
              aria-label={item.name}
            >
              <Image
                src={item.image}
                width={108}
                height={108}
                alt=""
                className="h-27 w-full object-contain"
              />
              <span className="w-full">{item.name}</span>
              <span className="flex items-center gap-1 text-xs text-muted-foreground">
                {selected.id === item.id && <CheckIcon aria-hidden="true" />}
                {selected.id === item.id ? '已选 · ' : ''}
                {item.categoryLabel}
              </span>
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      ) : (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>没有找到这件衣服</EmptyTitle>
            <EmptyDescription>
              试试衣物名称或颜色，当前选择已保留。
            </EmptyDescription>
          </EmptyHeader>
          <Button variant="ghost" onClick={() => onQueryChange('')}>
            清除搜索
          </Button>
        </Empty>
      )}
    </>
  )
}
