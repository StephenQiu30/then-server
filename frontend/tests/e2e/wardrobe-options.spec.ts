import { expect, test, type Page } from '@playwright/test'

const user = {
  id: '018f1f74-a2d0-7c6d-9c17-4a0ea2400a12',
  revision: 1,
  email: 'lin@example.test',
  display_name: '林杉',
  role: 'user',
  status: 'active',
  created_at: '2026-09-25T00:00:00Z',
  updated_at: '2026-09-25T00:00:00Z',
}

const item = {
  id: '018f1f74-a2d0-7c6d-9c17-4a0ea2400a13',
  revision: 1,
  name: '测试外套',
  category: 'outerwear',
  availability: 'laundry',
  source: 'wardrobe',
  lifecycle: 'active',
  attributes: {},
  created_at: '2026-09-25T00:00:00Z',
  updated_at: '2026-09-25T00:00:00Z',
}

async function mockWardrobe(page: Page) {
  await page.route('**/users/me', (route) =>
    route.fulfill({ status: 200, json: user }),
  )
  await page.route(/\/wardrobe\/items(?:\?.*)?$/, (route) =>
    route.fulfill({ status: 200, json: { items: [item] } }),
  )
}

test('a plan can select an unavailable wardrobe item and confirm it', async ({
  page,
}) => {
  await mockWardrobe(page)
  await page.route(/\/outfit-plans(?:\?.*)?$/, (route) =>
    route.fulfill({ status: 200, json: { plans: [] } }),
  )
  await page.goto('/plans')

  await page.getByRole('button', { name: '外套', exact: true }).click()
  const itemChoice = page.getByRole('button', { name: '选择测试外套' })
  await expect(itemChoice).toBeVisible()
  await itemChoice.click()
  await expect(itemChoice).toHaveAttribute('aria-pressed', 'true')

  const confirmation = page.getByRole('checkbox', {
    name: '测试外套当前待洗，仍加入计划',
  })
  await confirmation.check()
  await expect(confirmation).toBeChecked()

  await page.getByRole('button', { name: '移除测试外套' }).click()
  await expect(confirmation).toHaveCount(0)
})

test('the deletion impact dialog offers keyboard accessible history policies', async ({
  page,
}) => {
  await mockWardrobe(page)
  await page.route(`**/wardrobe/items/${item.id}/deletion-impact`, (route) =>
    route.fulfill({
      status: 200,
      json: {
        affected_plan_count: 1,
        affected_wear_event_count: 0,
        expected_impact: 'test-impact',
      },
    }),
  )
  await page.goto('/wardrobe')
  await page.getByRole('button', { name: '删除', exact: true }).click()

  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toBeVisible()
  const keepHistory = dialog.getByRole('radio', {
    name: '保留记录，清除这件衣物的快照',
  })
  const deleteHistory = dialog.getByRole('radio', {
    name: '同时删除关联计划与实际记录',
  })
  await expect(keepHistory).toBeChecked()
  await deleteHistory.check()
  await expect(deleteHistory).toBeChecked()
  await expect(keepHistory).not.toBeChecked()
})

test('wear records stay first and the editor preserves a partial entry when collapsed', async ({
  page,
}) => {
  await mockWardrobe(page)
  await page.route(/\/outfit-plans(?:\?.*)?$/, (route) =>
    route.fulfill({ status: 200, json: { plans: [] } }),
  )
  await page.route(/\/wear-events(?:\?.*)?$/, (route) =>
    route.fulfill({ status: 200, json: { events: [] } }),
  )
  await page.route('**/statistics/wear?*', (route) =>
    route.fulfill({
      status: 200,
      json: { wear_days: 0, wear_events: 0, item_uses: [] },
    }),
  )
  await page.goto('/wear')
  await expect(
    page.getByRole('heading', { name: '穿过的，才算数。' }),
  ).toBeVisible()
  await expect(page.getByText('不必一次记完整。')).toBeVisible()
  await page.getByRole('button', { name: '记录实际穿着', exact: true }).click()
  await expect(page.getByRole('button', { name: '收起记录表单' })).toBeVisible()
  await page.getByLabel('实际日期', { exact: true }).fill('2026-10-01')
  await page.getByRole('button', { name: '收起记录表单' }).click()
  await page.getByRole('button', { name: '添加一条记录' }).click()
  await expect(page.getByLabel('实际日期', { exact: true })).toHaveValue(
    '2026-10-01',
  )
})
