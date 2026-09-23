'use client'

import { useEffect, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { AccountShell } from '@/components/account/account-shell'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { useCurrentUser } from '@/hooks/account/use-current-user'
import { useAccountActions } from '@/hooks/account/use-account-actions'
import { accountErrorMessage, accountStatus } from '@/lib/account/errors'
import { registrationErrors } from '@/lib/account/validation'

type Mode = 'login' | 'register'
type FieldName = 'displayName' | 'email' | 'password' | 'confirmation'

export function AuthForm({ mode }: { mode: Mode }) {
  const router = useRouter()
  const actions = useAccountActions()
  const currentUser = useCurrentUser()
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [fieldErrors, setFieldErrors] = useState<
    Partial<Record<FieldName, string>>
  >({})
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const registering = mode === 'register'

  useEffect(() => {
    if (currentUser.data) router.replace('/account')
  }, [currentUser.data, router])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending) return
    const validation = registering
      ? registrationErrors({ displayName, email, password, confirmation })
      : {
          ...(!email.trim() ? { email: '请输入邮箱。' } : {}),
          ...(!password ? { password: '请输入密码。' } : {}),
        }
    const emailInput = event.currentTarget.elements.namedItem(
      'email',
    ) as HTMLInputElement | null
    if (emailInput?.validity.typeMismatch)
      validation.email = '请输入有效的邮箱地址。'
    setFieldErrors(validation)
    if (Object.keys(validation).length) return
    setPending(true)
    setError(null)
    try {
      if (registering) {
        await actions.register({
          display_name: displayName.trim(),
          email: email.trim(),
          password,
        })
      } else {
        await actions.login({ email: email.trim(), password })
      }
      setPassword('')
      setConfirmation('')
      router.replace('/account')
    } catch (cause) {
      setPassword('')
      setConfirmation('')
      if (registering && accountStatus(cause) === 409) {
        setFieldErrors({ email: '这个邮箱已被使用。' })
      }
      setError(accountErrorMessage(cause))
    } finally {
      setPending(false)
    }
  }

  const title = registering ? '创建账户' : '欢迎回来'
  const description = registering
    ? '用邮箱创建账户，管理你主动保存的资料。'
    : '登录后查看和管理你的账户资料。'
  return (
    <AccountShell title={title} description={description}>
      {currentUser.isPending || currentUser.data ? (
        <p
          role="status"
          className="flex items-center gap-3 text-muted-foreground"
        >
          <Spinner />
          正在检查会话…
        </p>
      ) : currentUser.isError && accountStatus(currentUser.error) !== 401 ? (
        <Alert>
          <AlertTitle>暂时无法检查会话</AlertTitle>
          <AlertDescription className="flex flex-col items-start gap-3">
            {accountErrorMessage(currentUser.error)}
            <Button
              type="button"
              variant="outline"
              onClick={() => void currentUser.refetch()}
            >
              重试
            </Button>
          </AlertDescription>
        </Alert>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>
              {registering ? '填写注册信息' : '输入账户信息'}
            </CardTitle>
            <CardDescription>
              {registering
                ? '注册后会直接建立当前会话。'
                : '使用你的邮箱和密码继续。'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              onSubmit={(event) => void submit(event)}
              noValidate
              className="flex flex-col gap-6"
            >
              <FieldGroup>
                {registering && (
                  <Field data-invalid={!!fieldErrors.displayName}>
                    <FieldLabel htmlFor="display-name">显示名称</FieldLabel>
                    <Input
                      id="display-name"
                      name="name"
                      autoComplete="name"
                      required
                      value={displayName}
                      onChange={(event) => setDisplayName(event.target.value)}
                      aria-invalid={!!fieldErrors.displayName}
                      aria-describedby={
                        fieldErrors.displayName
                          ? 'display-name-error'
                          : undefined
                      }
                    />
                    {fieldErrors.displayName && (
                      <FieldError id="display-name-error">
                        {fieldErrors.displayName}
                      </FieldError>
                    )}
                  </Field>
                )}
                <Field data-invalid={!!fieldErrors.email}>
                  <FieldLabel htmlFor="email">邮箱</FieldLabel>
                  <Input
                    id="email"
                    name="email"
                    type="email"
                    autoComplete="email"
                    required
                    maxLength={254}
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    aria-invalid={!!fieldErrors.email}
                    aria-describedby={
                      fieldErrors.email ? 'email-error' : undefined
                    }
                  />
                  {fieldErrors.email && (
                    <FieldError id="email-error">
                      {fieldErrors.email}
                    </FieldError>
                  )}
                </Field>
                <Field data-invalid={!!fieldErrors.password}>
                  <FieldLabel htmlFor="password">密码</FieldLabel>
                  <Input
                    id="password"
                    name="password"
                    type="password"
                    required
                    autoComplete={
                      registering ? 'new-password' : 'current-password'
                    }
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    aria-invalid={!!fieldErrors.password}
                    aria-describedby={
                      [
                        registering && 'password-help',
                        fieldErrors.password && 'password-error',
                      ]
                        .filter(Boolean)
                        .join(' ') || undefined
                    }
                  />
                  {registering && (
                    <FieldDescription id="password-help">
                      12–72 个 UTF-8 字节。
                    </FieldDescription>
                  )}
                  {fieldErrors.password && (
                    <FieldError id="password-error">
                      {fieldErrors.password}
                    </FieldError>
                  )}
                </Field>
                {registering && (
                  <Field data-invalid={!!fieldErrors.confirmation}>
                    <FieldLabel htmlFor="confirmation">确认密码</FieldLabel>
                    <Input
                      id="confirmation"
                      name="confirm-password"
                      type="password"
                      autoComplete="new-password"
                      required
                      value={confirmation}
                      onChange={(event) => setConfirmation(event.target.value)}
                      aria-invalid={!!fieldErrors.confirmation}
                      aria-describedby={
                        fieldErrors.confirmation
                          ? 'confirmation-error'
                          : undefined
                      }
                    />
                    {fieldErrors.confirmation && (
                      <FieldError id="confirmation-error">
                        {fieldErrors.confirmation}
                      </FieldError>
                    )}
                  </Field>
                )}
              </FieldGroup>
              {error && (
                <Alert variant="destructive" role="alert">
                  <AlertTitle>操作未完成</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}
              <Button type="submit" disabled={pending} className="w-full">
                {pending && <Spinner data-icon="inline-start" />}
                {pending ? '正在提交…' : title}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}
      <p className="mt-6 text-muted-foreground">
        {registering ? '已有账户？' : '还没有账户？'}{' '}
        <Link
          className="text-primary underline underline-offset-4"
          href={registering ? '/login' : '/register'}
        >
          {registering ? '登录' : '创建账户'}
        </Link>
      </p>
    </AccountShell>
  )
}
