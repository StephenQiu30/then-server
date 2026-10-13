'use client'

import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { PersonPreview } from '@/components/person/person-preview'
import { LookEmptyState } from '@/components/look/look-empty-state'
import { LookFollowUpActions } from '@/components/look/look-follow-up-actions'
import { SaveLookDialog } from '@/components/look/save-look-dialog'
import { useLookDraft } from '@/hooks/look/use-look-draft'
import { sampleLookFromDraft } from '@/lib/look/samples'
import { PageContainer } from '@/layout/page-container'
import { PageHeader } from '@/layout/page-header'

export function LookDetailView() {
  const { draft } = useLookDraft()
  const look = draft ? sampleLookFromDraft(draft) : null
  return (
    <PageContainer className="gap-8">
      <PageHeader
        title={look?.name || '搭配详情'}
        description="示例草稿 · 保存在当前浏览器"
        actions={
          <Button asChild variant="outline">
            <Link href="/looks">返回搭配收藏</Link>
          </Button>
        }
      />
      <div className="grid gap-6 min-[900px]:grid-cols-2">
        {look ? (
          <article className="rounded-xl bg-surface-muted p-6">
            <div className="relative h-[420px]">
              <PersonPreview
                src={look.image.src}
                alt={look.image.alt}
                priority
              />
            </div>
          </article>
        ) : (
          <LookEmptyState />
        )}
        <section className="flex flex-col items-start justify-center gap-6 py-6 min-[700px]:px-6">
          <h2 className="text-4xl font-medium leading-snug tracking-tight">
            喜欢的，
            <br />
            下次再穿。
          </h2>
          <p className="max-w-sm text-sm leading-6 text-muted-foreground">
            {look?.scene || '给搭配留一个名字，在需要的时候找到它。'}
          </p>
          {look && (
            <SaveLookDialog>
              <Button variant="outline">编辑收藏信息</Button>
            </SaveLookDialog>
          )}
          <Button asChild>
            <Link href="/">继续选衣试穿</Link>
          </Button>
          <LookFollowUpActions />
        </section>
      </div>
    </PageContainer>
  )
}
