import Link from 'next/link'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from '@/components/ui/empty'

export function LookEmptyState() {
  return (
    <Empty variant="muted" className="min-h-[420px]">
      <EmptyHeader>
        <EmptyTitle>
          <h2>把喜欢的搭配，留给下次。</h2>
        </EmptyTitle>
        <EmptyDescription>
          还没有收藏。先到试衣间看看，比较之后再留下喜欢的那套。
        </EmptyDescription>
      </EmptyHeader>
      <EmptyContent>
        <Button asChild>
          <Link href="/">去试衣间看看</Link>
        </Button>
      </EmptyContent>
    </Empty>
  )
}
