'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Field,
  FieldLabel,
  FieldDescription,
  FieldError,
} from '@/components/ui/field'
import { Spinner } from '@/components/ui/spinner'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { WardrobeItemImage } from '@/components/wardrobe/wardrobe-item-image'
import { useWardrobePhotoSource } from '@/hooks/wardrobe/use-wardrobe-photo'
import type { WardrobeForm } from '@/lib/wardrobe/form'
import { garmentSamples } from '@/lib/wardrobe/samples'
import { wardrobeErrorMessage } from '@/lib/wardrobe/errors'
import { emptyCrop, type CropInsets } from '@/lib/wardrobe/image-processing'

export function WardrobePhotoEditor({
  form,
  pending,
  onChange,
  onBusy,
}: {
  form: WardrobeForm
  pending: boolean
  onChange: (patch: Partial<WardrobeForm>) => void
  onBusy: (busy: boolean) => void
}) {
  const source = useWardrobePhotoSource()
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const [crop, setCrop] = useState<CropInsets>(emptyCrop)
  const hasSource = !!(form.photoFile || form.photoMediaID)
  const originalID = form.originalPhotoMediaID ?? form.photoMediaID
  const changed =
    !!(form.photoFile && form.photoFile !== form.photoOriginalFile) ||
    (!!form.photoMediaID && !!originalID && form.photoMediaID !== originalID)
  async function process(mode: 'remove' | 'crop' | 'restore') {
    if (pending || busy) return
    setBusy(true)
    onBusy(true)
    setError(null)
    setStatus(
      mode === 'remove'
        ? '正在本机加载模型并抠图…'
        : mode === 'crop'
          ? '正在裁剪…'
          : '正在恢复原图…',
    )
    const started = performance.now()
    try {
      const originalSource = form.photoOriginalFile
        ? { file: form.photoOriginalFile, legacy: false }
        : await source.read(originalID!, true)
      const original = originalSource.file
      const current =
        form.photoFile ??
        (form.photoMediaID === originalID
          ? original
          : (await source.read(form.photoMediaID!, false)).file)
      const tools = await import('@/lib/wardrobe/image-processing')
      const result =
        mode === 'restore'
          ? original
          : mode === 'remove'
            ? await tools.removeBackground(current)
            : await tools.cropPhoto(current, crop)
      onChange({
        photoOriginalFile: original,
        photoFile: result,
        photoPreview: URL.createObjectURL(result),
      })
      setCrop(emptyCrop)
      setStatus(
        `${originalSource.legacy ? '旧图原始上传版本已清理，本次保留已有展示图。' : ''}${mode === 'remove' ? '本地抠图' : mode === 'crop' ? '裁剪' : '恢复原图'}完成，用时 ${((performance.now() - started) / 1000).toFixed(1)} 秒；保存后用于搭配。`,
      )
    } catch (cause) {
      setStatus('')
      setError(wardrobeErrorMessage(cause))
    } finally {
      setBusy(false)
      onBusy(false)
    }
  }
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor="wardrobe-photo">衣物照片</FieldLabel>
      <Input
        key={form.photoMediaID ?? 'new'}
        id="wardrobe-photo"
        type="file"
        accept="image/jpeg,image/png,image/webp"
        disabled={pending || busy}
        aria-describedby="wardrobe-photo-description"
        onChange={(event) => {
          const file = event.target.files?.[0]
          if (!file) return
          setError(null)
          setStatus('')
          setCrop(emptyCrop)
          if (
            !['image/jpeg', 'image/png', 'image/webp'].includes(file.type) ||
            file.size > 12 * 1024 * 1024 ||
            file.size === 0
          ) {
            setError('请选择 12 MB 以内的 JPG、PNG 或 WebP 图片。')
            event.target.value = ''
            return
          }
          onChange({
            photoFile: file,
            photoOriginalFile: file,
            photoPreview: URL.createObjectURL(file),
            originalPhotoPreview: URL.createObjectURL(file),
            photoMediaID: undefined,
            originalPhotoMediaID: undefined,
          })
        }}
      />
      <FieldDescription id="wardrobe-photo-description">
        JPG、PNG 或 WebP，最多 12
        MB。抠图和裁剪在本机完成，原图单独保留；保存前可恢复。
      </FieldDescription>
      {(hasSource || form.sampleImage) && (
        <div
          className={
            changed ? 'grid grid-cols-2 gap-3' : 'mx-auto w-full max-w-64'
          }
        >
          {changed && (
            <div className="min-w-0">
              <p className="mb-2 text-sm text-muted-foreground">原图</p>
              <WardrobeItemImage
                item={{ name: '衣物原图' }}
                previewSrc={
                  form.originalPhotoPreview ??
                  (originalID
                    ? `/media/${originalID}/wardrobe-original`
                    : undefined)
                }
              />
            </div>
          )}
          <div className="min-w-0">
            {changed && (
              <p className="mb-2 text-sm text-muted-foreground">当前展示图</p>
            )}
            <WardrobeItemImage
              item={{
                name: form.name || '衣物照片预览',
                photo_media_id: form.photoMediaID,
                sample_image: form.sampleImage,
              }}
              previewSrc={form.photoPreview}
            />
          </div>
        </div>
      )}
      {hasSource && (
        <>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={pending || busy}
              onClick={() => void process('remove')}
            >
              自动抠图
            </Button>
            <Button
              type="button"
              variant="ghost"
              disabled={pending || busy}
              onClick={() => void process('restore')}
            >
              恢复原图
            </Button>
            {originalID && (
              <Button asChild variant="ghost">
                <a
                  href={`/media/${originalID}/wardrobe-original`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  查看原图
                </a>
              </Button>
            )}
          </div>
          <details className="rounded-lg border border-border p-3">
            <summary className="cursor-pointer text-sm font-medium">
              简单裁剪
            </summary>
            <p className="mt-2 text-sm text-muted-foreground">
              从四边移除空白或背景，单位为百分比；不会裁改原图。
            </p>
            <div className="my-3 grid grid-cols-2 gap-3">
              {(
                [
                  ['top', '上边'],
                  ['right', '右边'],
                  ['bottom', '下边'],
                  ['left', '左边'],
                ] as const
              ).map(([side, label]) => (
                <Field key={side}>
                  <FieldLabel htmlFor={`crop-${side}`}>{label}（%）</FieldLabel>
                  <Input
                    id={`crop-${side}`}
                    type="number"
                    min={0}
                    max={45}
                    step={1}
                    value={crop[side]}
                    disabled={pending || busy}
                    onChange={(e) =>
                      setCrop({ ...crop, [side]: Number(e.target.value) })
                    }
                  />
                </Field>
              ))}
            </div>
            <Button
              type="button"
              variant="outline"
              disabled={
                pending ||
                busy ||
                Object.values(crop).every((value) => value === 0)
              }
              onClick={() => void process('crop')}
            >
              应用裁剪
            </Button>
          </details>
        </>
      )}
      {status && (
        <p
          role="status"
          className="flex items-center gap-2 text-sm text-muted-foreground"
        >
          {busy && <Spinner />}
          {status}
        </p>
      )}
      {error && <FieldError role="alert">{error}</FieldError>}
      {!hasSource && (
        <details>
          <summary className="cursor-pointer text-sm text-muted-foreground">
            使用现有示例图
          </summary>
          <NativeSelect
            aria-label="示例图片"
            value={form.sampleImage ?? ''}
            disabled={pending}
            onChange={(e) =>
              onChange({
                sampleImage:
                  (e.target.value as WardrobeForm['sampleImage']) || undefined,
              })
            }
          >
            <NativeSelectOption value="">暂不设置图片</NativeSelectOption>
            {garmentSamples.map((sample) => (
              <NativeSelectOption key={sample.key} value={sample.key}>
                {sample.name}（示例）
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <p className="mt-2 text-sm text-muted-foreground">
            Codex 示例素材，非实物照片。
          </p>
        </details>
      )}
    </Field>
  )
}
