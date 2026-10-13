import { test, expect, type Page } from '@playwright/test'
import path from 'node:path'

test('a failed person image is recoverable without losing the outfit selection', async ({
  page,
}) => {
  let blockImage = true
  await page.route('**/look-samples/original.png', async (route) => {
    if (blockImage) await route.abort()
    else await route.continue()
  })
  await page.goto('/')
  const stage = page.getByRole('region', { name: '人物展示' })
  await expect(
    stage.getByText('照片暂时未能加载，选衣内容已保留。'),
  ).toBeVisible()
  blockImage = false
  await stage.getByRole('button', { name: '重新加载照片' }).click()
  await expect(stage.getByRole('img')).toBeVisible()
  await expect(stage.getByRole('alert')).toHaveCount(0)
  await expect(
    page.getByRole('button', { name: '查看对比示例', exact: true }),
  ).toBeEnabled()
})

async function readyImages(page: Page) {
  await expect(page.getByRole('status', { name: '正在加载对比' })).toHaveCount(
    0,
  )
  await expect(async () => {
    const images = await page.locator('main img:visible').evaluateAll((nodes) =>
      nodes.map((node) => ({
        loaded: (node as HTMLImageElement).complete,
        width: (node as HTMLImageElement).naturalWidth,
      })),
    )
    expect(images.length).toBeGreaterThan(0)
    expect(images.every((image) => image.loaded && image.width > 0)).toBe(true)
  }).toPass()
}

test('sample selection is exact, and comparison supports keyboard, drag and local saving', async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  const requests: string[] = []
  const errors: string[] = []
  page.on('request', (request) => {
    if (request.method() === 'POST') requests.push(request.url())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto('/')
  await readyImages(page)
  await page.screenshot({
    path: testInfo.outputPath('room-desktop.png'),
    fullPage: true,
  })
  await page.getByRole('button', { name: '试穿这件上衣' }).click()
  await expect(page.getByRole('dialog')).toContainText('真实试穿尚未接入')
  await page.getByRole('button', { name: '返回调整' }).click()
  await page.getByRole('radio', { name: '下装', exact: true }).click()
  await expect(
    page.getByRole('radio', { name: '经典直筒牛仔裤', exact: true }),
  ).toHaveAttribute('aria-checked', 'true')
  await page.getByRole('radio', { name: '上装', exact: true }).click()
  await page.getByRole('radio', { name: '海军蓝棉质衬衫', exact: true }).click()
  await expect(
    page
      .getByRole('radiogroup', { name: '选择一件示例衣物' })
      .getByRole('radio', { checked: true }),
  ).toHaveCount(1)
  await expect(page.getByRole('button', { name: '试穿这件上衣' })).toHaveCSS(
    'background-color',
    'rgb(23, 23, 23)',
  )
  await page.getByRole('button', { name: '查看对比示例', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: '新的搭配，还是你。' }),
  ).toBeFocused()
  await readyImages(page)
  await page.screenshot({
    path: testInfo.outputPath('comparison-desktop.png'),
    fullPage: true,
  })
  await page.getByRole('radio', { name: '滑动对比' }).click()
  const slider = page.getByRole('slider')
  await slider.press('ArrowRight')
  await expect(slider).toHaveAttribute('aria-valuenow', '51')
  const track = await page.locator('[data-slot="slider"]').boundingBox()
  if (!track) throw new Error('Comparison slider is missing')
  await page.mouse.move(
    track.x + track.width * 0.51,
    track.y + track.height / 2,
  )
  await page.mouse.down()
  await page.mouse.move(
    track.x + track.width * 0.8,
    track.y + track.height / 2,
    { steps: 5 },
  )
  await page.mouse.up()
  expect(Number(await slider.getAttribute('aria-valuenow'))).toBeGreaterThan(70)
  await page.getByRole('button', { name: '收藏这套示例' }).click()
  await page.getByRole('textbox', { name: '搭配名称' }).fill('周五蓝调')
  await page.getByRole('button', { name: '保存示例草稿' }).click()
  await expect(page.getByRole('heading', { name: '周五蓝调' })).toBeVisible()
  await page.reload()
  await page.getByRole('button', { name: '查看对比示例', exact: true }).click()
  await expect(page.getByRole('heading', { name: '周五蓝调' })).toBeVisible()
  await page.getByRole('button', { name: '查看环绕预览' }).click()
  await expect(page.getByRole('dialog')).toContainText('尚无连续环绕素材')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: '查看环绕预览' })).toBeFocused()
  expect(requests).toEqual([])
  expect(errors).toEqual([])
})

test('an uploaded photo remains local and cannot be presented as a prepared try-on result', async ({
  page,
}) => {
  await page.goto('/')
  await page.getByRole('button', { name: '更换照片' }).click()
  const input = page.getByLabel('选择全身照片')
  await input.setInputFiles({
    name: 'bad.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('not an image'),
  })
  await expect(page.getByText('请选择 JPG、PNG 或 WebP 图片。')).toBeVisible()
  await input.setInputFiles(path.resolve('public/look-samples/original.png'))
  await expect(page.getByRole('button', { name: '使用这张照片' })).toBeEnabled()
  await page.getByRole('button', { name: '使用这张照片' }).click()
  await expect(
    page.getByRole('img', { name: '我的全身照片预览' }),
  ).toHaveAttribute('src', /^blob:/)
  await page.getByRole('button', { name: '试穿这件上衣' }).click()
  await expect(page.getByRole('dialog')).toContainText('你的照片只在本地预览')
  await expect(page.getByRole('dialog')).toContainText('没有提交生成任务')
  await page.getByRole('button', { name: '返回调整' }).click()
  await page.getByRole('button', { name: '返回示例人物' }).click()
  await expect(
    page.getByRole('button', { name: '查看对比示例', exact: true }),
  ).toBeEnabled()
})

test('mobile flow and all layout breakpoints retain visible content without horizontal overflow', async ({
  page,
}, testInfo) => {
  for (const width of [
    320, 390, 699, 700, 768, 834, 859, 860, 1023, 1024, 1199, 1200, 1400, 1440,
  ]) {
    await page.setViewportSize({ width, height: 1000 })
    await page.goto('/')
    await readyImages(page)
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
      `room ${width}px`,
    ).toBe(true)
    await page
      .getByRole('button', { name: '查看对比示例', exact: true })
      .click()
    await readyImages(page)
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
      `comparison ${width}px`,
    ).toBe(true)
  }
  await page.setViewportSize({ width: 390, height: 1000 })
  await page.goto('/')
  await readyImages(page)
  await page.screenshot({
    path: testInfo.outputPath('room-mobile.png'),
    fullPage: true,
  })
  await page.getByRole('button', { name: '查看对比示例', exact: true }).click()
  await page.getByRole('radio', { name: '原穿搭', exact: true }).click()
  await expect(
    page.getByRole('img', { name: '原穿搭：米白针织与炭灰长裤', exact: true }),
  ).toBeVisible()
  await page.getByRole('radio', { name: '试穿效果', exact: true }).click()
  await readyImages(page)
  await page.screenshot({
    path: testInfo.outputPath('comparison-mobile.png'),
    fullPage: true,
  })
  await page.getByRole('button', { name: '重新选择衣物' }).click()
  await page.getByRole('button', { name: '打开导航菜单' }).click()
  await expect(
    page
      .getByRole('navigation', { name: '移动端导航' })
      .getByRole('link', { name: '衣橱', exact: true }),
  ).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: '打开导航菜单' })).toBeFocused()
  await page.addStyleTag({ content: 'html { font-size: 200%; }' })
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
})

test('saved sample is recoverable in collection and stays separate from real wear', async ({
  page,
}) => {
  await page.goto('/looks')
  await expect(
    page.getByRole('heading', { name: '把喜欢的搭配，留给下次。' }),
  ).toBeVisible()
  await page.getByRole('link', { name: '去试衣间看看' }).click()
  await page.getByRole('button', { name: '查看对比示例', exact: true }).click()
  await page.getByRole('button', { name: '收藏这套示例' }).click()
  await page.getByRole('textbox', { name: '搭配名称' }).fill('测试收藏')
  await page.getByRole('button', { name: '保存示例草稿' }).click()
  await page.getByRole('link', { name: '查看搭配收藏' }).click()
  await expect(page).toHaveURL(/\/looks$/)
  await expect(page.getByRole('heading', { name: '测试收藏' })).toBeVisible()
  await page.reload()
  await page.getByRole('link', { name: '查看这套搭配' }).click()
  await expect(page).toHaveURL(/\/looks\/sample$/)
  await expect(page.getByRole('heading', { name: '测试收藏' })).toBeVisible()
  await expect(page.getByText('收藏不代表穿过')).toBeVisible()
  await expect(
    page.getByRole('link', { name: '记录实际穿着' }),
  ).toHaveAttribute('href', '/wear')
})

test('storage failure preserves the save form and does not claim success', async ({
  page,
}) => {
  await page.addInitScript(() => {
    Storage.prototype.setItem = () => {
      throw new Error('Unavailable')
    }
  })
  await page.goto('/')
  await page.getByRole('button', { name: '查看对比示例', exact: true }).click()
  await page.getByRole('button', { name: '收藏这套示例' }).click()
  await page.getByRole('textbox', { name: '搭配名称' }).fill('保存失败保留')
  await page.getByRole('button', { name: '保存示例草稿' }).click()
  await expect(page.getByRole('dialog')).toContainText('浏览器未能保存草稿')
  await expect(page.getByRole('textbox', { name: '搭配名称' })).toHaveValue(
    '保存失败保留',
  )
})

test('a draft saved under the previous brand survives and migrates on save', async ({
  page,
}) => {
  const previousKey = 'chuanjian.try-on.sample-draft.v1'
  const currentKey = 'wearlog.try-on.sample-draft.v1'
  await page.addInitScript(
    ({ previousKey, currentKey }) => {
      if (
        !localStorage.getItem(previousKey) &&
        !localStorage.getItem(currentKey)
      ) {
        localStorage.setItem(
          previousKey,
          JSON.stringify({ name: '品牌迁移前的搭配', scene: '通勤' }),
        )
      }
    },
    { previousKey, currentKey },
  )
  await page.goto('/looks/sample')
  await expect(
    page.getByRole('heading', { name: '品牌迁移前的搭配' }),
  ).toBeVisible()
  await page.getByRole('button', { name: '编辑收藏信息' }).click()
  await page
    .getByRole('textbox', { name: '搭配名称' })
    .fill('WEARLOG 保留的搭配')
  await page.getByRole('button', { name: '保存示例草稿' }).click()
  await expect(
    page.getByRole('heading', { name: 'WEARLOG 保留的搭配' }),
  ).toBeVisible()
  expect(
    await page.evaluate((key) => localStorage.getItem(key), previousKey),
  ).toBeNull()
  await page.reload()
  await expect(
    page.getByRole('heading', { name: 'WEARLOG 保留的搭配' }),
  ).toBeVisible()
})

test('direct clothing search retains selection through empty results and keyboard switching', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 960 })
  await page.goto('/')
  const search = page.getByRole('searchbox', { name: '搜索示例衣橱' })
  await search.fill('针织')
  await page.getByRole('radio', { name: '米白针织衫', exact: true }).click()
  await search.fill('不存在的单品')
  await expect(page.getByText('没有找到这件衣服')).toBeVisible()
  await expect(
    page.getByText('当前选择已保留。', { exact: false }),
  ).toBeVisible()
  await page.getByRole('button', { name: '清除搜索' }).click()
  const knit = page.getByRole('radio', { name: '米白针织衫', exact: true })
  await expect(knit).toHaveAttribute('aria-checked', 'true')
  await knit.focus()
  await knit.press('ArrowLeft')
  await expect(
    page.getByRole('radio', { name: '海军蓝棉质衬衫', exact: true }),
  ).toBeFocused()
  await page.keyboard.press('Space')
  await expect(
    page.getByRole('radio', { name: '海军蓝棉质衬衫', exact: true }),
  ).toHaveAttribute('aria-checked', 'true')
  await expect(
    page.getByRole('img', { name: '炭灰垂感长裤', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('img', { name: '白色低帮运动鞋', exact: true }),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: '试穿这件上衣' })).toHaveCSS(
    'border-top-color',
    'rgba(0, 0, 0, 0)',
  )
  const hitArea = await page
    .getByRole('button', { name: '试穿这件上衣' })
    .boundingBox()
  expect(hitArea?.height).toBeGreaterThanOrEqual(44)
})

test('mobile clothing sheet supports search, selection, dismissal and focus return', async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  const trigger = page.getByRole('button', { name: '换一件', exact: true })
  await trigger.click()
  const sheet = page.getByRole('dialog')
  await expect(sheet).toContainText('这次，想换哪一件？')
  const search = sheet.getByRole('searchbox', { name: '搜索示例衣橱' })
  await search.fill('牛仔')
  await sheet
    .getByRole('radio', { name: '经典直筒牛仔裤', exact: true })
    .click()
  await page.screenshot({
    path: testInfo.outputPath('mobile-clothing-sheet.png'),
    fullPage: true,
  })
  await sheet.getByRole('button', { name: '使用这件衣服' }).click()
  await expect(page.getByRole('button', { name: '试穿这件下装' })).toBeVisible()
  await expect(trigger).toBeFocused()
  await trigger.click()
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  await expect(page.getByRole('button', { name: '试穿这件下装' })).toBeVisible()
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
})
