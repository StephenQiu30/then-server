export type PlanSelection = API.OutfitSelectionRequest

export type PlanForm = {
  localDate: string
  timeZone: string
  contextSummary: string
  items: PlanSelection[]
  confirmedUnavailableIDs: string[]
}

export function todayInTimeZone(timeZone: string): string {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date())
  const value = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  const year = value.year
  const month = value.month
  const day = value.day
  return `${year}-${month}-${day}`
}

export function emptyPlanForm(): PlanForm {
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  return {
    localDate: todayInTimeZone(timeZone),
    timeZone,
    contextSummary: '',
    items: [],
    confirmedUnavailableIDs: [],
  }
}

export function planFormFromResponse(plan: API.OutfitPlanResponse): PlanForm {
  const items = plan.items as API.OutfitPlanItemResponse[]
  return {
    localDate: plan.local_date,
    timeZone: plan.time_zone,
    contextSummary: plan.context_summary ?? '',
    items: items.flatMap(({ content }) =>
      content
        ? [{ item_id: content.item_id, revision: content.item_revision }]
        : [],
    ),
    confirmedUnavailableIDs: [],
  }
}

export function planFormError(
  form: PlanForm,
  originalDate?: string,
): string | null {
  const date = new Date(`${form.localDate}T00:00:00`)
  if (
    !/^\d{4}-\d{2}-\d{2}$/.test(form.localDate) ||
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== Number(form.localDate.slice(0, 4)) ||
    date.getMonth() + 1 !== Number(form.localDate.slice(5, 7)) ||
    date.getDate() !== Number(form.localDate.slice(8, 10))
  ) {
    return '请选择有效的计划日期。'
  }
  if (!form.timeZone) return '无法读取设备时区，请刷新页面重试。'
  if (
    form.localDate < todayInTimeZone(form.timeZone) &&
    form.localDate !== originalDate
  ) {
    return '计划日期不能早于今天。'
  }
  if (form.items.length < 1 || form.items.length > 20) {
    return '请选择 1 至 20 件真实衣物。'
  }
  const summary = form.contextSummary.trim()
  if (
    Array.from(summary).length > 120 ||
    /[\u0000-\u001f\u007f-\u009f]/u.test(summary)
  ) {
    return '场景说明不能超过 120 个字符或包含控制字符。'
  }
  return null
}

export function planRequestFields(form: PlanForm) {
  return {
    local_date: form.localDate,
    time_zone: form.timeZone,
    context_summary: form.contextSummary.trim() || undefined,
    items: form.items,
    confirmed_unavailable_ids: form.confirmedUnavailableIDs,
  }
}
