import { todayInTimeZone } from '@/lib/outfit-plan/form'

export function currentWearRange() {
  const today = todayInTimeZone(
    Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
  )
  return { from: `${today.slice(0, 4)}-01-01`, to: today }
}
