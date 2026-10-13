'use client'

import Image from 'next/image'
import { useState } from 'react'
import { ImageIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { sampleImagePath } from '@/lib/wardrobe/samples'

export function WardrobeItemImage({
  item,
  className,
  decorative = false,
  eager = false,
  previewSrc,
}: {
  item: Pick<
    API.WardrobeItemResponse,
    'name' | 'sample_image' | 'photo_media_id'
  >
  previewSrc?: string
  className?: string
  decorative?: boolean
  eager?: boolean
}) {
  const src =
    previewSrc ??
    (item.photo_media_id
      ? `/media/${item.photo_media_id}/wardrobe-image`
      : sampleImagePath(item.sample_image))
  const [failedSource, setFailedSource] = useState<string | null>(null)
  return (
    <div
      aria-hidden={decorative || undefined}
      className={cn(
        'relative flex aspect-square items-center justify-center overflow-hidden rounded-lg bg-muted/40',
        className,
      )}
    >
      {src && failedSource !== src ? (
        <Image
          src={src}
          unoptimized={!!previewSrc || !!item.photo_media_id}
          alt={decorative ? '' : item.name}
          fill
          loading={eager ? 'eager' : 'lazy'}
          sizes="(max-width: 640px) 45vw, 260px"
          className="object-contain p-4"
          onError={() => setFailedSource(src)}
        />
      ) : (
        <span className="flex flex-col items-center gap-2 p-4 text-center text-sm text-muted-foreground">
          <ImageIcon className="size-6" aria-hidden="true" />
          暂无图片
        </span>
      )}
    </div>
  )
}
