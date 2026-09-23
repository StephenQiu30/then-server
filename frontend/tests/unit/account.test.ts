import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  accountErrorMessage,
  accountStatus,
} from '../../src/lib/account/errors.ts'
import {
  passwordLengthValid,
  registrationErrors,
} from '../../src/lib/account/validation.ts'

test('password limits count UTF-8 bytes, including multibyte characters', () => {
  assert.equal(passwordLengthValid('123456789012'), true)
  assert.equal(passwordLengthValid('12345678901'), false)
  assert.equal(passwordLengthValid('你好你好你好你好你好你好'), true)
  assert.equal(passwordLengthValid('你'.repeat(25)), false)
})

test('registration rejects mismatch without retaining a false field error', () => {
  assert.deepEqual(
    registrationErrors({
      displayName: ' 于是 ',
      email: 'a@example.test',
      password: 'long-password-2026',
      confirmation: 'other-password',
    }),
    {
      confirmation: '两次输入的密码不一致。',
    },
  )
})

test('display name limit counts characters instead of UTF-16 code units', () => {
  assert.deepEqual(
    registrationErrors({
      displayName: '🧥'.repeat(80),
      email: 'a@example.test',
      password: 'long-password-2026',
      confirmation: 'long-password-2026',
    }),
    {},
  )
})

test('account errors expose safe recovery text without server details', () => {
  const conflict = {
    response: {
      status: 409,
      data: { message: 'database unique index users_email' },
    },
  }
  assert.equal(accountStatus(conflict), 409)
  assert.equal(
    accountErrorMessage(conflict),
    '资料已变化或邮箱已被使用，请核对后重试。',
  )
  assert.equal(
    accountErrorMessage(new Error('sensitive endpoint')),
    '连接失败，请检查网络后重试。',
  )
})
