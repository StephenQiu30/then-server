import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { PersonPreview } from '@/components/person/person-preview'
import type { LookSummary } from '@/lib/look/model'

export function LookCard({ look }: { look: LookSummary }) {
  return (
    <article className="flex flex-col gap-5 rounded-xl bg-surface-muted p-6">
      <div className="relative h-[420px]">
        <PersonPreview src={look.image.src} alt={look.image.alt} priority />
      </div>
      <div className="flex flex-col gap-2">
        <h2 className="text-2xl font-medium">{look.name}</h2>
        <p className="text-sm text-muted-foreground">
          {look.scene || '未填写场景'} · {look.sourceLabel}
        </p>
      </div>
      <Button asChild variant="outline">
        <Link href={look.href}>查看这套搭配</Link>
      </Button>
    </article>
  )
}
