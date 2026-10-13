'use client'

import { useEffect, useRef, useState, type ReactNode } from 'react'
import Image from 'next/image'
import dynamic from 'next/dynamic'
import Link from 'next/link'
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  HeartIcon,
  ImagePlusIcon,
  PlusIcon,
  Rotate3DIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { PersonPhotoDialog } from '@/components/person/person-photo-dialog'
import { PersonPreview } from '@/components/person/person-preview'
import { GarmentPicker } from '@/components/wardrobe/garment-picker'
import { sampleLookComparison } from '@/lib/look/samples'
import { Spinner } from '@/components/ui/spinner'
import { SaveLookDialog } from '@/components/look/save-look-dialog'
import { useLookDraft } from '@/hooks/look/use-look-draft'
import {
  originalGarments,
  fittingRoomGarments,
} from '@/lib/fitting-room/samples'
import type { GarmentOption } from '@/lib/wardrobe/selection'

const LookComparison = dynamic(
  () =>
    import('@/components/look/look-comparison').then(
      (module) => module.LookComparison,
    ),
  {
    loading: () => (
      <div
        role="status"
        aria-label="正在加载对比"
        className="flex min-h-96 items-center justify-center gap-2"
      >
        <Spinner />
        <span>正在打开前后对比…</span>
      </div>
    ),
  },
)

function OrbitPreviewDialog({ children }: { children: ReactNode }) {
  return (
    <Dialog>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>环绕预览尚未就绪</DialogTitle>
          <DialogDescription>
            这套示例只有正面照片，尚无连续环绕素材。你可以先比较前后穿搭；这里不会把照片翻转当作
            360° 展示。
          </DialogDescription>
        </DialogHeader>
      </DialogContent>
    </Dialog>
  )
}

export function FittingRoomView() {
  const [unavailable, setUnavailable] = useState(false)
  const primaryAction = useRef<HTMLButtonElement | null>(null)
  const [view, setView] = useState<'room' | 'result'>('room')
  const [selected, setSelected] = useState<GarmentOption>(
    fittingRoomGarments[0],
  )
  const [photo, setPhoto] = useState<string | null>(null)
  const heading = useRef<HTMLHeadingElement>(null)
  const { draft } = useLookDraft()
  useEffect(
    () => () => {
      if (photo) URL.revokeObjectURL(photo)
    },
    [photo],
  )

  function navigate(next: 'room' | 'result') {
    setView(next)
    window.scrollTo({ top: 0, behavior: 'instant' })
    requestAnimationFrame(() => heading.current?.focus({ preventScroll: true }))
  }
  function reset() {
    setPhoto(null)
    setSelected(fittingRoomGarments[0])
  }
  function ownPhoto(file: File) {
    setPhoto(URL.createObjectURL(file))
    navigate('room')
  }

  return (
    <div className="mx-auto w-full max-w-[1440px]">
      {view === 'room' ? (
        <div className="grid min-w-0 min-[700px]:min-h-[calc(100svh-4rem)] min-[700px]:grid-cols-[minmax(0,1fr)_280px] min-[1100px]:grid-cols-[164px_minmax(0,1fr)_304px]">
          <aside
            aria-label="我的穿搭"
            className="hidden flex-col gap-3 p-4 min-[1100px]:flex"
          >
            <h2 className="text-xs leading-[18px] text-muted-foreground">
              YOUR LOOKS / {photo ? '01' : '02'}
            </h2>
            <ToggleGroup
              type="single"
              value="original"
              orientation="vertical"
              variant="tile"
              spacing={3}
              className="w-full"
              aria-label="选择穿搭"
              onValueChange={(value) => {
                if (value === 'result') navigate('result')
              }}
            >
              <ToggleGroupItem
                value="original"
                className="w-full"
                aria-label="原穿搭"
              >
                <Image
                  src={photo ?? sampleLookComparison.before.src}
                  unoptimized={!!photo}
                  width={116}
                  height={96}
                  alt="原穿搭缩略图"
                  className="h-24 w-full object-contain"
                />
                <span>{photo ? '我的穿搭' : '轻松通勤'}</span>
                <span className="text-xs text-muted-foreground">
                  LOOK 01 · 原穿搭
                </span>
              </ToggleGroupItem>
              {!photo && (
                <ToggleGroupItem
                  value="result"
                  className="w-full"
                  aria-label="蓝调日常对比示例"
                >
                  <Image
                    src={sampleLookComparison.after.src}
                    width={116}
                    height={96}
                    alt="试穿 01 缩略图"
                    className="h-24 w-full object-contain"
                  />
                  <span className="max-w-full truncate">
                    {draft?.name ?? '蓝调日常'}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    LOOK 02 · 示例
                  </span>
                </ToggleGroupItem>
              )}
            </ToggleGroup>
            <Button variant="surface" className="w-full" onClick={reset}>
              <PlusIcon data-icon="inline-start" aria-hidden="true" />
              新的搭配
            </Button>
            <Button asChild variant="ghost" className="w-full">
              <Link href="/looks">搭配收藏</Link>
            </Button>
          </aside>
          <section
            aria-label="人物展示"
            className="flex min-w-0 flex-col bg-surface-muted p-5 pb-0 min-[700px]:bg-background min-[700px]:p-6"
          >
            <div className="flex min-h-20 items-start justify-between gap-3 min-[700px]:min-h-29">
              <div className="flex min-w-0 flex-col gap-2">
                <p className="order-2 text-xs leading-[18px] text-muted-foreground min-[700px]:order-none">
                  {photo ? '我的形象 / 仅本地预览' : 'LOOK 01 / 轻松通勤'}
                </p>
                <h1
                  ref={heading}
                  tabIndex={-1}
                  className="text-2xl leading-[34px] font-medium tracking-tight outline-none min-[700px]:text-[32px] min-[700px]:leading-[44px]"
                >
                  <span className="min-[700px]:hidden">今天，穿出新感觉。</span>
                  <span className="hidden min-[700px]:inline">
                    轻松的，
                    <br />
                    也可以很有型。
                  </span>
                </h1>
              </div>
              <PersonPhotoDialog onPhoto={ownPhoto}>
                <Button variant="surface" aria-label="更换照片">
                  <ImagePlusIcon data-icon="inline-start" aria-hidden="true" />
                  <span className="hidden min-[700px]:inline">更换形象</span>
                </Button>
              </PersonPhotoDialog>
            </div>
            <div className="relative h-96 min-[700px]:h-[clamp(24rem,calc(100svh-23.5rem),36.5rem)]">
              <PersonPreview
                key={photo ?? 'sample'}
                src={photo ?? sampleLookComparison.before.src}
                alt={
                  photo
                    ? '我的全身照片预览'
                    : '示例人物原穿搭：米白针织、炭灰长裤与白色运动鞋'
                }
                priority
                sizes="(min-width: 1100px) 390px, (min-width: 700px) 40vw, 90vw"
              />
            </div>
            <div className="flex justify-center">
              <OrbitPreviewDialog>
                <Button variant="ghost" aria-label="查看环绕预览">
                  <ArrowLeftIcon data-icon="inline-start" aria-hidden="true" />
                  环绕 · 待验证
                  <ArrowRightIcon data-icon="inline-end" aria-hidden="true" />
                </Button>
              </OrbitPreviewDialog>
            </div>
            <ToggleGroup
              type="single"
              value="original"
              variant="line"
              className="hidden self-center min-[700px]:flex"
              aria-label="人物视图"
              onValueChange={(value) => {
                if (value === 'result') navigate('result')
              }}
            >
              <ToggleGroupItem value="original">原穿搭</ToggleGroupItem>
              <ToggleGroupItem value="result">对比示例</ToggleGroupItem>
            </ToggleGroup>
            {photo && (
              <Button variant="ghost" onClick={reset}>
                返回示例人物
              </Button>
            )}
          </section>
          <GarmentPicker
            items={fittingRoomGarments}
            selected={selected}
            preserved={[
              selected.category === 'top'
                ? originalGarments.bottom
                : originalGarments.top,
              originalGarments.shoes,
            ]}
            onSelected={setSelected}
            action={
              <Button
                className="flex-1 min-[700px]:w-full"
                ref={primaryAction}
                onClick={() => setUnavailable(true)}
              >
                试穿这件{selected.category === 'top' ? '上衣' : '下装'}
              </Button>
            }
            secondaryAction={
              <Button
                variant="surface"
                className="w-full"
                onClick={() => navigate('result')}
              >
                查看对比示例
                <ArrowRightIcon data-icon="inline-end" aria-hidden="true" />
              </Button>
            }
            helperText="真实试穿尚未接入，选择不会发起生成。"
          />
        </div>
      ) : (
        <div className="grid min-w-0 gap-6 px-5 py-6 min-[700px]:p-8 min-[1100px]:grid-cols-[minmax(0,1fr)_304px]">
          <div className="flex min-w-0 flex-col gap-4">
            <div className="flex flex-col gap-2">
              <h1
                ref={heading}
                tabIndex={-1}
                className="text-2xl leading-[34px] font-medium tracking-tight outline-none min-[700px]:text-[32px] min-[700px]:leading-[44px]"
              >
                新的搭配，还是你。
              </h1>
              <p className="text-sm leading-[22px] text-muted-foreground">
                对比示例 · 既有成套图片，与本次单件选择无关。
              </p>
            </div>
            <LookComparison {...sampleLookComparison} />
          </div>
          <aside
            className="flex min-w-0 flex-col gap-4 min-[1100px]:gap-5 min-[1100px]:pt-24"
            aria-label="试穿结果操作"
          >
            <h2 className="hidden text-2xl leading-[34px] font-medium min-[700px]:block">
              值得留下的，
              <br />
              是新的可能。
            </h2>
            <h3 className="text-lg leading-[26px] font-medium">
              {draft?.name ?? '蓝调日常'}
            </h3>
            <figure className="hidden w-31 flex-col gap-1 rounded-sm bg-selection p-2 min-[700px]:flex">
              <Image
                src={fittingRoomGarments[0].image}
                width={108}
                height={108}
                alt="示例中的海军蓝棉质衬衫"
                className="h-27 w-full object-contain"
              />
              <figcaption className="text-xs leading-[18px]">
                海军蓝棉质衬衫
              </figcaption>
              <p className="text-xs leading-[18px] text-muted-foreground">
                成套效果示意
              </p>
            </figure>
            <p className="text-sm leading-[22px] text-muted-foreground">
              比较配色与整体比例。
              <br />
              收藏不会增加穿着次数。
            </p>
            <SaveLookDialog>
              <Button className="w-full">
                <HeartIcon data-icon="inline-start" aria-hidden="true" />
                {draft ? '编辑已存草稿' : '收藏这套示例'}
              </Button>
            </SaveLookDialog>
            <Button
              variant="surface"
              className="w-full"
              onClick={() => navigate('room')}
            >
              重新选择衣物
            </Button>
            <Button asChild variant="ghost" className="w-full">
              <Link href="/looks">查看搭配收藏</Link>
            </Button>
            <Button asChild variant="ghost" className="w-full">
              <Link href="/wear">记录实际穿着</Link>
            </Button>
            <OrbitPreviewDialog>
              <Button variant="ghost" className="w-full">
                <Rotate3DIcon data-icon="inline-start" aria-hidden="true" />
                查看环绕预览
              </Button>
            </OrbitPreviewDialog>
          </aside>
        </div>
      )}
      <Dialog open={unavailable} onOpenChange={setUnavailable}>
        <DialogContent
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            primaryAction.current?.focus()
          }}
        >
          <DialogHeader>
            <DialogTitle>真实试穿尚未接入</DialogTitle>
            <DialogDescription>
              {photo ? '你的照片只在本地预览。' : ''}已保留所选的
              {selected.name}
              ，没有提交生成任务。现有对比示例同时改变了上衣和裤子，不代表本次单件试穿结果。
            </DialogDescription>
          </DialogHeader>
          <Button onClick={() => setUnavailable(false)}>返回调整</Button>
          <Button
            variant="surface"
            onClick={() => {
              setUnavailable(false)
              navigate('result')
            }}
          >
            查看对比示例
          </Button>
        </DialogContent>
      </Dialog>
    </div>
  )
}
