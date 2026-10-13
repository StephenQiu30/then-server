'use client'

import { useState } from 'react'
import Image from 'next/image'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

export function PersonPreview({
  src,
  alt,
  priority = false,
  sizes = '(min-width: 1024px) 45vw, 90vw',
}: {
  src: string
  alt: string
  priority?: boolean
  sizes?: string
}) {
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [attempt, setAttempt] = useState(0)
  if (status === 'error')
    return (
      <div className="absolute inset-0 flex items-center justify-center p-4">
        <Alert className="max-w-xs">
          <AlertDescription>
            照片暂时未能加载，选衣内容已保留。
          </AlertDescription>
          <Button
            variant="ghost"
            onClick={() => {
              setAttempt(attempt + 1)
              setStatus('loading')
            }}
          >
            重新加载照片
          </Button>
        </Alert>
      </div>
    )
  return (
    <>
      {status === 'loading' && (
        <div
          role="status"
          className="absolute inset-0 flex items-center justify-center"
        >
          <Spinner />
          <span className="sr-only">正在加载人物照片</span>
        </div>
      )}
      <Image
        key={attempt}
        src={src}
        alt={alt}
        fill
        sizes={sizes}
        loading={priority ? undefined : 'eager'}
        className="object-contain"
        priority={priority}
        unoptimized
        onLoad={() => setStatus('ready')}
        onError={() => setStatus('error')}
      />
    </>
  )
}
