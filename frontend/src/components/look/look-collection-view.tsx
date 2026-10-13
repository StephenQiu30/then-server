'use client'

import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { LookCard } from '@/components/look/look-card'
import { LookEmptyState } from '@/components/look/look-empty-state'
import { LookFollowUpActions } from '@/components/look/look-follow-up-actions'
import { useLookDraft } from '@/hooks/look/use-look-draft'
import { sampleLookFromDraft } from '@/lib/look/samples'
import { PageContainer } from '@/layout/page-container'
import { PageHeader } from '@/layout/page-header'

export function LookCollectionView() {
  const { draft } = useLookDraft()
  return (
    <PageContainer className="gap-8">
      <PageHeader
        title="搭配收藏"
        description="收藏是搭配，记录是穿过。想穿的时候，再回来。"
        actions={
          <Button asChild>
            <Link href="/">开始新的试穿</Link>
          </Button>
        }
      />
      <div className="grid gap-6 min-[700px]:grid-cols-2 min-[1100px]:grid-cols-3">
        {draft ? (
          <LookCard look={sampleLookFromDraft(draft)} />
        ) : (
          <LookEmptyState />
        )}
        <section className="flex flex-col items-start justify-center gap-6 py-6 min-[700px]:px-6">
          <h2 className="text-4xl font-medium leading-snug tracking-tight">
            旧衣服，
            <br />
            新的可能。
          </h2>
          <p className="max-w-sm text-sm leading-6 text-muted-foreground">
            从已经拥有的衣服开始，给日常多一点新的选择。
          </p>
          <Button asChild>
            <Link href="/">进入试衣间</Link>
          </Button>
          <LookFollowUpActions />
        </section>
      </div>
    </PageContainer>
  )
}
