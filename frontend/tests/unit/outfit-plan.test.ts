import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  planFormError,
  planFormFromResponse,
  planRequestFields,
  type PlanForm,
} from '../../src/lib/outfit-plan/form.ts'

test('plan form reads snapshots without reviving redacted clothes', () => {
  const form = planFormFromResponse({
    local_date: '2026-09-23',
    time_zone: 'Asia/Shanghai',
    context_summary: '通勤',
    items: [
      { ordinal: 0, content: null },
      { ordinal: 1, content: { item_id: 'shirt', item_revision: 3 } },
    ],
  } as unknown as API.OutfitPlanResponse)
  assert.deepEqual(form.items, [{ item_id: 'shirt', revision: 3 }])
  assert.deepEqual(form.confirmedUnavailableIDs, [])
})

test('plan validation keeps past dates only when editing their original date', () => {
  const form: PlanForm = {
    localDate: '2020-01-01',
    timeZone: 'Asia/Shanghai',
    contextSummary: '🧥'.repeat(120),
    items: [{ item_id: 'shirt', revision: 1 }],
    confirmedUnavailableIDs: [],
  }
  assert.match(planFormError(form) ?? '', /不能早于今天/)
  assert.equal(planFormError(form, '2020-01-01'), null)
  form.contextSummary += '🧥'
  assert.match(planFormError(form, '2020-01-01') ?? '', /120/)
  form.contextSummary = '周末\n散步'
  assert.match(planFormError(form, '2020-01-01') ?? '', /控制字符/)
})

test('empty context is omitted from the plan request', () => {
  const fields = planRequestFields({
    localDate: '2026-09-23',
    timeZone: 'Asia/Shanghai',
    contextSummary: '  ',
    items: [{ item_id: 'shirt', revision: 2 }],
    confirmedUnavailableIDs: [],
  })
  assert.equal(fields.context_summary, undefined)
})
