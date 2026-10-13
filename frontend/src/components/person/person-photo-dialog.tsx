'use client'

import { useEffect, useRef, useState, type ReactNode } from 'react'
import { ImagePlusIcon } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { PersonPreview } from '@/components/person/person-preview'
import { photoValidationError } from '@/lib/person/photo'
import { sampleLookComparison } from '@/lib/look/samples'

export function PersonPhotoDialog({
  children,
  onPhoto,
}: {
  children: ReactNode
  onPhoto: (file: File) => void
}) {
  const [open, setOpen] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [checking, setChecking] = useState(false)
  const sequence = useRef(0)
  useEffect(
    () => () => {
      if (preview) URL.revokeObjectURL(preview)
    },
    [preview],
  )

  async function choosePhoto(next: File | undefined) {
    if (!next) return
    const request = ++sequence.current
    const message = photoValidationError(next)
    setFile(null)
    setPreview(null)
    setError(message)
    if (message) return
    setChecking(true)
    const url = URL.createObjectURL(next)
    try {
      const image = new window.Image()
      image.src = url
      await image.decode()
      if (request !== sequence.current) {
        URL.revokeObjectURL(url)
        return
      }
      setPreview(url)
      setFile(next)
    } catch {
      URL.revokeObjectURL(url)
      if (request === sequence.current)
        setError('无法读取这张照片，请换一张清晰的图片。')
    } finally {
      if (request === sequence.current) setChecking(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) {
          sequence.current++
          setFile(null)
          setPreview(null)
          setError('')
          setChecking(false)
        }
      }}
    >
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>添加自己的形象</DialogTitle>
          <DialogDescription>
            一张清晰的全身穿搭照，就能开始。
          </DialogDescription>
        </DialogHeader>
        <div className="relative h-64 rounded-lg bg-surface-muted">
          <PersonPreview
            key={preview ?? 'sample'}
            src={preview ?? sampleLookComparison.before.src}
            alt={preview ? '待确认的全身照片' : '全身照片示例'}
            sizes="360px"
          />
        </div>
        <Field data-invalid={!!error}>
          <FieldLabel htmlFor="person-photo">
            <ImagePlusIcon aria-hidden="true" className="size-4" />
            选择全身照片
          </FieldLabel>
          <Input
            id="person-photo"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            aria-invalid={!!error}
            aria-describedby="person-photo-help person-photo-error"
            onChange={(event) => {
              void choosePhoto(event.target.files?.[0])
              event.target.value = ''
            }}
          />
          <FieldDescription id="person-photo-help">
            人物完整入镜，光线清晰，避免大面积遮挡。JPG / PNG / WebP，最大 10
            MB。
          </FieldDescription>
          <FieldError id="person-photo-error">{error}</FieldError>
        </Field>
        <p className="text-xs text-muted-foreground">
          照片仅在本页预览，不上传。自己的照片试穿尚未开放。
        </p>
        <Button
          disabled={!file || checking}
          onClick={() => {
            if (file) {
              onPhoto(file)
              setOpen(false)
              setFile(null)
              setPreview(null)
            }
          }}
        >
          {checking ? '正在检查照片…' : '使用这张照片'}
        </Button>
      </DialogContent>
    </Dialog>
  )
}
