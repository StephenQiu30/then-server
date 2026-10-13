'use client'

import { useState } from 'react'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Slider } from '@/components/ui/slider'
import { PersonPreview } from '@/components/person/person-preview'
import type { LookImage } from '@/lib/look/model'

export function LookComparison({
  before,
  after,
}: {
  before: LookImage
  after: LookImage
}) {
  const [mode, setMode] = useState('side-by-side')
  const [mobileView, setMobileView] = useState('result')
  const [split, setSplit] = useState(50)
  const images = { original: before, result: after }
  const activeImage = images[mobileView === 'original' ? 'original' : 'result']
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <ToggleGroup
        type="single"
        variant="solid"
        value={mobileView}
        onValueChange={(value) => {
          if (value) setMobileView(value)
        }}
        className="w-full min-[700px]:hidden"
        aria-label="人物前后对比"
      >
        <ToggleGroupItem value="original" className="flex-1">
          {before.label}
        </ToggleGroupItem>
        <ToggleGroupItem value="result" className="flex-1">
          {after.label}
        </ToggleGroupItem>
      </ToggleGroup>
      <div className="bg-surface-muted min-[700px]:hidden">
        <div className="relative h-[min(31rem,60svh)]">
          <PersonPreview
            key={mobileView}
            src={activeImage.src}
            alt={activeImage.alt}
            sizes="90vw"
          />
        </div>
      </div>
      {mode === 'side-by-side' ? (
        <div className="hidden grid-cols-2 gap-6 min-[700px]:grid">
          {(['original', 'result'] as const).map((view) => (
            <figure
              key={view}
              className="flex h-[clamp(28rem,calc(100svh-20rem),40rem)] min-w-0 flex-col gap-2 bg-surface-muted p-5"
            >
              <figcaption className="text-xs leading-[18px] text-muted-foreground">
                {images[view].label}
                {view === 'result' ? ' · 效果示意' : ''}
              </figcaption>
              <div className="relative max-h-139 min-h-0 flex-1">
                <PersonPreview
                  src={images[view].src}
                  alt={images[view].alt}
                  sizes="(min-width: 1280px) 371px, 36vw"
                />
              </div>
            </figure>
          ))}
        </div>
      ) : (
        <div className="hidden flex-col gap-4 bg-surface-muted p-5 min-[700px]:flex">
          <div className="flex justify-between gap-4 text-xs text-muted-foreground">
            <span>{before.label}</span>
            <span>{after.label} · 效果示意</span>
          </div>
          <div
            className="relative h-[clamp(24rem,calc(100svh-25.25rem),34.75rem)]"
            data-testid="slider-comparison"
          >
            <PersonPreview src={after.src} alt={'滑动对比：' + after.label} />
            <div
              className="absolute inset-0 bg-surface-muted"
              style={{ clipPath: 'inset(0 ' + (100 - split) + '% 0 0)' }}
            >
              <PersonPreview
                src={before.src}
                alt={'滑动对比：' + before.label}
              />
            </div>
            <div
              className="pointer-events-none absolute inset-y-0 w-px bg-foreground/50"
              style={{ left: split + '%' }}
            />
          </div>
          <Slider
            min={0}
            max={100}
            step={1}
            value={[split]}
            onValueChange={(values) => setSplit(values[0])}
            aria-label="拖动比较原穿搭和试穿效果"
            className="min-h-11"
          />
          <p className="text-center text-xs text-muted-foreground">
            左右拖动，对比配色与整体比例
          </p>
        </div>
      )}
      <ToggleGroup
        type="single"
        variant="solid"
        value={mode}
        onValueChange={(value) => {
          if (value) setMode(value)
        }}
        aria-label="对比方式"
        className="hidden self-center min-[700px]:flex"
      >
        <ToggleGroupItem value="side-by-side" className="min-w-35">
          并排对比
        </ToggleGroupItem>
        <ToggleGroupItem value="slider" className="min-w-35">
          滑动对比
        </ToggleGroupItem>
      </ToggleGroup>
    </div>
  )
}
