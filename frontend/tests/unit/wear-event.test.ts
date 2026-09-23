import assert from 'node:assert/strict'
import { test } from 'node:test'
import { duplicateCandidates } from '../../src/lib/wear-event/errors.ts'
import {
  wearFormError,
  followedPlanError,
  wearFormFromResponse,
  wearRequestFields,
  type WearForm,
} from '../../src/lib/wear-event/form.ts'

function form(): WearForm {
  return {
    localDate: '2020-01-01',
    timeZone: 'Asia/Shanghai',
    completeness: 'partial',
    contextSummary: '',
    sourceKind: 'unplanned',
    sourcePlanID: '',
    sourcePlanRevision: null,
    items: [{ item_id: 'shirt', revision: 2 }],
    confirmedUnavailableIDs: [],
    laundryItemIDs: [],
    duplicateConfirmations: [],
  }
}

test('wear form validates source relationship and preserves partial fact', () => {
  const value = form()
  assert.equal(wearFormError(value), null)
  assert.equal(wearRequestFields(value).source_plan_id, undefined)
  value.sourceKind = 'followed_plan'
  assert.match(wearFormError(value) ?? '', /来源计划/)
  value.sourcePlanID = 'plan'
  value.sourcePlanRevision = 3
  assert.equal(wearFormError(value), null)
  value.sourceKind = 'unplanned'
  assert.match(wearFormError(value) ?? '', /不能关联/)
})

test('followed plan requires the complete original wardrobe set', () => {
  const value = form()
  value.sourceKind = 'followed_plan'
  value.sourcePlanID = 'plan'
  value.sourcePlanRevision = 1
  const plan = {
    items: [{ ordinal: 0, content: { item_id: 'shirt' } }],
  } as API.OutfitPlanResponse
  assert.equal(followedPlanError(value, plan), null)
  value.items.push({ item_id: 'jacket', revision: 1 })
  assert.match(followedPlanError(value, plan) ?? '', /换了衣物/)
  plan.items = [{ ordinal: 0, content: null }]
  assert.match(followedPlanError(value, plan) ?? '', /已清除衣物/)
})

test('redacted historical item never becomes a new selection', () => {
  const mapped = wearFormFromResponse({
    local_date: '2020-01-01',
    time_zone: 'Asia/Shanghai',
    completeness: 'complete',
    source_kind: 'unplanned',
    items: [
      { ordinal: 0, content: null },
      { ordinal: 1, content: { item_id: 'shirt', item_revision: 2 } },
    ],
  } as unknown as API.WearEventResponse)
  assert.deepEqual(mapped.items, [{ item_id: 'shirt', revision: 2 }])
  assert.deepEqual(mapped.laundryItemIDs, [])
})

test('only structured duplicate candidates enable explicit confirmation', () => {
  const valid = {
    response: {
      status: 409,
      data: { duplicate_candidates: [{ id: 'event', revision: 2 }] },
    },
  }
  assert.deepEqual(duplicateCandidates(valid), [{ id: 'event', revision: 2 }])
  assert.equal(
    duplicateCandidates({
      response: { status: 409, data: { duplicate_candidates: [{ id: 4 }] } },
    }),
    null,
  )
  assert.equal(
    duplicateCandidates({
      response: { status: 500, data: valid.response.data },
    }),
    null,
  )
})
