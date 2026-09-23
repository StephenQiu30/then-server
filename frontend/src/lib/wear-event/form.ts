import { todayInTimeZone } from '../outfit-plan/form.ts'

export type WearForm = {
  localDate: string
  timeZone: string
  completeness: API.CreateWearEventRequest['completeness']
  contextSummary: string
  sourceKind: API.CreateWearEventRequest['source_kind']
  sourcePlanID: string
  sourcePlanRevision: number | null
  items: API.OutfitSelectionRequest[]
  confirmedUnavailableIDs: string[]
  laundryItemIDs: string[]
  duplicateConfirmations: API.WearEventCandidateResponse[]
}

export function emptyWearForm(): WearForm {
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  return {
    localDate: todayInTimeZone(timeZone),
    timeZone,
    completeness: 'complete',
    contextSummary: '',
    sourceKind: 'unplanned',
    sourcePlanID: '',
    sourcePlanRevision: null,
    items: [],
    confirmedUnavailableIDs: [],
    laundryItemIDs: [],
    duplicateConfirmations: [],
  }
}

export function wearFormFromResponse(event: API.WearEventResponse): WearForm {
  return {
    localDate: event.local_date,
    timeZone: event.time_zone,
    completeness: event.completeness,
    contextSummary: event.context_summary ?? '',
    sourceKind: event.source_kind,
    sourcePlanID: event.source_plan_id ?? '',
    sourcePlanRevision: event.source_plan_revision ?? null,
    items: (event.items as API.OutfitPlanItemResponse[]).flatMap(
      ({ content }) =>
        content
          ? [{ item_id: content.item_id, revision: content.item_revision }]
          : [],
    ),
    confirmedUnavailableIDs: [],
    laundryItemIDs: [],
    duplicateConfirmations: [],
  }
}

export function wearFormError(form: WearForm): string | null {
  const date = new Date(`${form.localDate}T00:00:00`)
  if (
    !/^\d{4}-\d{2}-\d{2}$/.test(form.localDate) ||
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== Number(form.localDate.slice(0, 4)) ||
    date.getMonth() + 1 !== Number(form.localDate.slice(5, 7)) ||
    date.getDate() !== Number(form.localDate.slice(8, 10))
  )
    return '请选择有效的实际穿着日期。'
  if (!form.timeZone) return '无法读取设备时区，请刷新页面重试。'
  if (form.localDate > todayInTimeZone(form.timeZone))
    return '实际穿着日期不能晚于今天。'
  if (form.items.length < 1 || form.items.length > 20)
    return '请选择 1 至 20 件真实衣物。'
  if (
    new Set(form.items.map((item) => item.item_id)).size !== form.items.length
  )
    return '衣物不能重复选择。'
  if (
    form.sourceKind !== 'unplanned' &&
    (!form.sourcePlanID || !form.sourcePlanRevision)
  )
    return '请选择来源计划。'
  if (
    form.sourceKind === 'unplanned' &&
    (form.sourcePlanID || form.sourcePlanRevision)
  )
    return '无计划记录不能关联计划。'
  const summary = form.contextSummary.trim()
  if (
    Array.from(summary).length > 120 ||
    /[\u0000-\u001f\u007f-\u009f]/u.test(summary)
  ) {
    return '场景说明不能超过 120 个字符或包含控制字符。'
  }
  return null
}

export function followedPlanError(
  form: WearForm,
  plan: API.OutfitPlanResponse | undefined,
): string | null {
  if (form.sourceKind !== 'followed_plan' || !plan) return null
  const items = plan.items as API.OutfitPlanItemResponse[]
  if (items.some((item) => !item.content)) {
    return '来源计划含已清除衣物，请改为换件或无计划记录。'
  }
  const planned = items.map((item) => item.content.item_id).sort()
  const actual = form.items.map((item) => item.item_id).sort()
  if (
    planned.length !== actual.length ||
    planned.some((id, index) => id !== actual[index])
  ) {
    return '实际衣物与计划不同，请选“计划中换了衣物”。'
  }
  return null
}

export function wearRequestFields(form: WearForm) {
  return {
    local_date: form.localDate,
    time_zone: form.timeZone,
    completeness: form.completeness,
    context_summary: form.contextSummary.trim() || undefined,
    source_kind: form.sourceKind,
    source_plan_id: form.sourcePlanID || undefined,
    source_plan_revision: form.sourcePlanRevision ?? undefined,
    items: form.items,
    confirmed_unavailable_ids: form.confirmedUnavailableIDs,
    laundry_item_ids: form.laundryItemIDs,
    duplicate_confirmations: form.duplicateConfirmations,
  }
}
