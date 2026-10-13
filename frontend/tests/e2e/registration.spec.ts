import { expect, test, type Page } from '@playwright/test'

const registeredUser = {
  id: '018f1f74-a2d0-7c6d-9c17-4a0ea2400a12',
  revision: 1,
  email: 'lin@example.test',
  display_name: '林杉',
  role: 'user',
  status: 'active',
  created_at: '2026-09-25T00:00:00Z',
  updated_at: '2026-09-25T00:00:00Z',
}

async function showRegistrationForm(page: Page) {
  await page.route('**/users/me', (route) =>
    route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({ message: 'unauthenticated' }),
    }),
  )
  await page.goto('/register')
  await expect(page.getByRole('heading', { name: '创建账户' })).toBeVisible()
}

test('visitor can register and reach their account from the home page', async ({
  page,
}) => {
  let registrationBody: unknown
  let registered = false
  await page.route('**/users/me', (route) =>
    route.fulfill({
      status: registered ? 200 : 401,
      contentType: 'application/json',
      body: JSON.stringify(
        registered ? registeredUser : { message: 'unauthenticated' },
      ),
    }),
  )
  await page.route('**/auth/registrations', async (route) => {
    registrationBody = route.request().postDataJSON()
    registered = true
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ user: registeredUser }),
    })
  })

  await page.goto('/')
  await expect(
    page.getByRole('heading', { name: /轻松的，\s*也可以很有型/ }),
  ).toBeVisible()
  await page.getByRole('link', { name: '我的', exact: true }).click()
  await expect(page).toHaveURL(/\/login$/)
  await page.getByRole('link', { name: '创建账户' }).click()
  await expect(page).toHaveURL(/\/register$/)
  await expect(page.getByRole('heading', { name: '创建账户' })).toBeVisible()

  await page.getByLabel('显示名称').fill(' 林杉 ')
  await page.getByLabel('邮箱').fill('lin@example.test')
  await page.getByLabel('密码', { exact: true }).fill('correct-password')
  await page.getByLabel('确认密码').fill('correct-password')
  await page.getByRole('button', { name: '创建账户' }).click()

  await expect(page).toHaveURL(/\/account$/)
  await expect(page.getByRole('heading', { name: '我的账户' })).toBeVisible()
  await expect(page.getByLabel('显示名称')).toHaveValue('林杉')
  await expect(page.getByLabel('邮箱')).toHaveValue('lin@example.test')
  expect(registrationBody).toEqual({
    display_name: '林杉',
    email: 'lin@example.test',
    password: 'correct-password',
  })
})

test('registration rejects mismatched passwords before sending a request', async ({
  page,
}) => {
  let registrationRequests = 0
  await showRegistrationForm(page)
  await page.route('**/auth/registrations', async (route) => {
    registrationRequests += 1
    await route.abort()
  })

  await page.getByLabel('显示名称').fill('林杉')
  await page.getByLabel('邮箱').fill('lin@example.test')
  await page.getByLabel('密码', { exact: true }).fill('correct-password')
  await page.getByLabel('确认密码').fill('different-password')
  await page.getByRole('button', { name: '创建账户' }).click()

  await expect(page.getByText('两次输入的密码不一致。')).toBeVisible()
  expect(registrationRequests).toBe(0)
})

test('login and registration have a focused layout on desktop and mobile', async ({
  page,
}) => {
  await page.route('**/users/me', (route) =>
    route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({ message: 'unauthenticated' }),
    }),
  )

  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: '欢迎回来' })).toBeVisible()
  await expect(page.getByRole('link', { name: '衣序，返回首页' })).toBeVisible()
  await expect(page.locator('[data-slot="card"]')).toBeVisible()
  await expect(page.getByRole('navigation', { name: '主要导航' })).toHaveCount(
    0,
  )
  await expect(page.getByRole('navigation', { name: '页脚导航' })).toHaveCount(
    0,
  )
  await page.screenshot({ path: '/tmp/then-auth-login-desktop.png' })

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(
    page.getByRole('link', { name: '返回首页', exact: true }),
  ).toBeVisible()
  await expect(page.getByLabel('邮箱')).toBeVisible()
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
  await page.screenshot({ path: '/tmp/then-auth-login-mobile.png' })

  await page.getByRole('link', { name: '创建账户' }).click()
  await expect(page).toHaveURL(/\/register$/)
  await expect(page.getByLabel('确认密码')).toBeVisible()
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true)
  await page.screenshot({ path: '/tmp/then-auth-register-mobile.png' })

  await page.goto('/missing-auth-layout-check')
  await expect(page.locator('main#main-content')).toHaveCount(1)
  await expect(page.locator('header')).toHaveCount(1)
  await expect(page.locator('footer')).toHaveCount(1)
})
