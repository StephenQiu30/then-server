'use client'

import { useState } from 'react'
import { useWearStatistics } from '@/hooks/wear-event/use-wear-statistics'
import { currentWearRange } from '@/lib/wear-event/statistics'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

export function WearStatistics({ enabled }: { enabled: boolean }) {
  const [range] = useState(currentWearRange)
  const stats = useWearStatistics(enabled, range.from, range.to)
  if (!enabled) return null
  return (
    <section
      id="statistics"
      aria-label="实际穿着统计"
      className="flex flex-col gap-4 rounded-xl border border-border p-5"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-medium">今年的实际穿着</h2>
        <p className="text-sm text-muted-foreground">
          {range.from} 至 {range.to}
        </p>
      </div>
      {stats.isPending ? (
        <p role="status" className="flex items-center gap-2">
          <Spinner />
          正在读取统计…
        </p>
      ) : stats.isError ? (
        <div role="alert" className="flex flex-wrap items-center gap-3">
          <p>统计暂时无法读取。</p>
          <Button variant="outline" onClick={() => void stats.refetch()}>
            重试统计
          </Button>
        </div>
      ) : (
        <dl className="grid grid-cols-3 gap-4">
          {[
            [stats.data.wear_days, '穿着天数'],
            [stats.data.wear_events, '实穿记录'],
            [stats.data.item_uses.length, '穿过的单品'],
          ].map(([value, label]) => (
            <div key={label}>
              <dt className="text-sm text-muted-foreground">{label}</dt>
              <dd className="mt-1 text-2xl font-semibold tabular-nums">
                {value}
              </dd>
            </div>
          ))}
        </dl>
      )}
      <p className="text-sm text-muted-foreground">
        仅计算已确认的实穿；修改或删除记录后重新统计。
      </p>
    </section>
  )
}
