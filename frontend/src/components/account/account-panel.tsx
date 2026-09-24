'use client'

import { useEffect, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { AccountShell } from '@/components/account/account-shell'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { useCurrentUser } from '@/hooks/account/use-current-user'
import { useAccountActions } from '@/hooks/account/use-account-actions'
import { accountErrorMessage, accountStatus } from '@/lib/account/errors'

export function AccountPanel() {
  const router = useRouter()
  const actions = useAccountActions()
  const currentUser = useCurrentUser()
  const user = currentUser.data
  const userKey = user ? `${user.id}:${user.revision}` : ''
  const [draft, setDraft] = useState<{
    key: string
    displayName: string
    email: string
  } | null>(null)
  const displayName =
    draft?.key === userKey ? draft.displayName : (user?.display_name ?? '')
  const email = draft?.key === userKey ? draft.email : (user?.email ?? '')
  const [pending, setPending] = useState<'update' | 'logout' | 'delete' | null>(
    null,
  )
  const [error, setError] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)
  const [emailError, setEmailError] = useState<string | null>(null)
  const [deletion, setDeletion] = useState<API.AccountDeletionResponse | null>(
    null,
  )

  useEffect(() => {
    if (
      !deletion &&
      currentUser.isError &&
      accountStatus(currentUser.error) === 401
    ) {
      actions.clearCurrent()
      router.replace('/login')
    }
  }, [currentUser.error, currentUser.isError, deletion, router, actions])

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!user || pending) return
    if (!displayName.trim() || Array.from(displayName.trim()).length > 80) {
      setNameError('请输入不超过 80 个字符的显示名称。')
      return
    }
    if (!email.trim()) {
      setEmailError('请输入邮箱。')
      return
    }
    const emailInput = event.currentTarget.elements.namedItem(
      'email',
    ) as HTMLInputElement | null
    if (emailInput?.validity.typeMismatch) {
      setEmailError('请输入有效的邮箱地址。')
      return
    }
    setPending('update')
    setError(null)
    setNameError(null)
    setEmailError(null)
    try {
      await actions.update({
        display_name: displayName.trim(),
        email: email.trim(),
        expected_revision: user.revision,
      })
      toast('账户资料已保存。')
    } catch (cause) {
      if (accountStatus(cause) === 401) {
        actions.clearCurrent()
        router.replace('/login')
      } else {
        setError(accountErrorMessage(cause))
      }
    } finally {
      setPending(null)
    }
  }

  async function logout() {
    if (pending) return
    setPending('logout')
    setError(null)
    try {
      await actions.logout()
      router.replace('/login')
    } catch (cause) {
      setError(accountErrorMessage(cause))
      setPending(null)
    }
  }

  async function remove() {
    if (pending) return
    setPending('delete')
    setError(null)
    try {
      const receipt = await actions.remove()
      setDeletion(receipt)
    } catch (cause) {
      if (accountStatus(cause) === 401) {
        actions.clearCurrent()
        router.replace('/login')
      } else {
        setError(accountErrorMessage(cause))
      }
    } finally {
      setPending(null)
    }
  }

  if (deletion) {
    return (
      <AccountShell
        title="删除已受理"
        description="账户访问已关闭。若有私有媒体，清理可能仍在进行。"
      >
        <Alert role="status">
          <AlertTitle>删除请求已受理</AlertTitle>
          <AlertDescription>
            状态：{deletion.status === 'complete' ? '已完成' : '清理中'}
            。请保留回执编号：{deletion.id}
          </AlertDescription>
        </Alert>
        <Button asChild className="mt-6">
          <Link href="/register">返回注册</Link>
        </Button>
      </AccountShell>
    )
  }

  return (
    <AccountShell
      title="我的账户"
      description="查看和修改本人资料。所有变更以服务端保存结果为准。"
    >
      {currentUser.isPending ||
      (currentUser.isError && accountStatus(currentUser.error) === 401) ? (
        <p
          role="status"
          className="flex items-center gap-3 text-muted-foreground"
        >
          <Spinner />
          正在读取账户…
        </p>
      ) : currentUser.isError ? (
        <Alert>
          <AlertTitle>账户暂时无法读取</AlertTitle>
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
        user && (
          <div className="flex flex-col gap-6">
            <div className="flex flex-wrap gap-3">
              <Button asChild variant="outline">
                <Link href="/wardrobe">查看我的衣橱</Link>
              </Button>
              <Button asChild variant="outline">
                <Link href="/plans">查看穿搭计划</Link>
              </Button>
              <Button asChild variant="outline">
                <Link href="/wear">查看实际穿着</Link>
              </Button>
            </div>
            <Card>
              <CardHeader>
                <CardTitle>个人资料</CardTitle>
                <CardDescription>
                  创建于{' '}
                  {new Intl.DateTimeFormat('zh-CN', {
                    dateStyle: 'long',
                  }).format(new Date(user.created_at))}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(event) => void save(event)}
                  noValidate
                  className="flex flex-col gap-6"
                >
                  <FieldGroup>
                    <Field data-invalid={!!nameError}>
                      <FieldLabel htmlFor="account-name">显示名称</FieldLabel>
                      <Input
                        id="account-name"
                        name="name"
                        autoComplete="name"
                        required
                        value={displayName}
                        onChange={(event) =>
                          setDraft({
                            key: userKey,
                            displayName: event.target.value,
                            email,
                          })
                        }
                        aria-invalid={!!nameError}
                        aria-describedby={
                          nameError ? 'account-name-error' : undefined
                        }
                      />
                      {nameError && (
                        <FieldError id="account-name-error">
                          {nameError}
                        </FieldError>
                      )}
                    </Field>
                    <Field data-invalid={!!emailError}>
                      <FieldLabel htmlFor="account-email">邮箱</FieldLabel>
                      <Input
                        id="account-email"
                        name="email"
                        type="email"
                        autoComplete="email"
                        required
                        maxLength={254}
                        value={email}
                        onChange={(event) =>
                          setDraft({
                            key: userKey,
                            displayName,
                            email: event.target.value,
                          })
                        }
                        aria-invalid={!!emailError}
                        aria-describedby={
                          emailError ? 'account-email-error' : undefined
                        }
                      />
                      {emailError && (
                        <FieldError id="account-email-error">
                          {emailError}
                        </FieldError>
                      )}
                    </Field>
                  </FieldGroup>
                  {error && (
                    <Alert variant="destructive" role="alert">
                      <AlertTitle>操作未完成</AlertTitle>
                      <AlertDescription>{error}</AlertDescription>
                    </Alert>
                  )}
                  <div className="flex flex-wrap gap-3">
                    <Button type="submit" disabled={pending !== null}>
                      {pending === 'update' && (
                        <Spinner data-icon="inline-start" />
                      )}
                      {pending === 'update' ? '正在保存…' : '保存资料'}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={pending !== null}
                      onClick={() => {
                        setError(null)
                        setNameError(null)
                        setEmailError(null)
                        void currentUser.refetch()
                      }}
                    >
                      重新加载资料
                    </Button>
                  </div>
                </form>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>会话与删除</CardTitle>
                <CardDescription>
                  退出会撤销当前会话；删除会立即关闭账户访问。
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-wrap items-center gap-3">
                <Badge variant="secondary">当前会话</Badge>
                <Button
                  type="button"
                  variant="outline"
                  disabled={pending !== null}
                  onClick={() => void logout()}
                >
                  {pending === 'logout' ? '正在退出…' : '退出登录'}
                </Button>
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      type="button"
                      variant="destructive"
                      disabled={pending !== null}
                    >
                      删除账户
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>确认删除账户？</AlertDialogTitle>
                      <AlertDialogDescription>
                        全部会话会立即撤销，私有媒体清理可能继续进行。这个操作无法撤销。
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>保留账户</AlertDialogCancel>
                      <AlertDialogAction
                        variant="destructive"
                        onClick={() => void remove()}
                      >
                        确认删除
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </CardContent>
            </Card>
          </div>
        )
      )}
    </AccountShell>
  )
}
