import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

export function LookFollowUpActions() {
  return (
    <>
      <Alert variant="muted" role="note">
        <AlertTitle>收藏不代表穿过</AlertTitle>
        <AlertDescription>
          当前收藏为本地示例，不同步到账户、不关联真实衣物，也不会增加穿着次数。实际穿过后，请在记录中选择自己的衣物。
        </AlertDescription>
      </Alert>
      <div className="flex flex-wrap gap-3">
        <Button asChild variant="outline">
          <Link href="/wear">记录实际穿着</Link>
        </Button>
        <Button asChild variant="ghost">
          <Link href="/plans">查看穿搭计划</Link>
        </Button>
      </div>
    </>
  )
}
