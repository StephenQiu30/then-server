'use client'

import { useEffect, useRef, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
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
import { Field, FieldLabel, FieldLegend, FieldSet } from '@/components/ui/field'
import { PageShell } from '@/components/layout/page-shell'
import { Spinner } from '@/components/ui/spinner'
import { WardrobeEditor } from '@/components/wardrobe/wardrobe-editor'
import { useAccountActions } from '@/hooks/account/use-account-actions'
import { useCurrentUser } from '@/hooks/account/use-current-user'
import {
  useWardrobeActions,
  useWardrobeItems,
} from '@/hooks/wardrobe/use-wardrobe'
import { accountStatus } from '@/lib/account/errors'
import { wardrobeErrorMessage } from '@/lib/wardrobe/errors'
import {
  availabilityLabels,
  categoryLabels,
  emptyWardrobeForm,
  wardrobeFormFromItem,
  wardrobeNameError,
  type WardrobeForm,
} from '@/lib/wardrobe/form'

type DeleteDraft = {
  item: API.WardrobeItemResponse
  impact: API.WardrobeDeletionImpactResponse
  policy: 'redact_snapshots' | 'delete_affected_history'
}

export function WardrobePanel() {
  const router = useRouter()
  const accountActions = useAccountActions()
  const currentUser = useCurrentUser()
  const wardrobe = useWardrobeItems(!!currentUser.data)
  const actions = useWardrobeActions()
  const createID = useRef<string | null>(null)
  const [form, setForm] = useState<WardrobeForm>(emptyWardrobeForm)
  const [editing, setEditing] = useState<API.WardrobeItemResponse | null>(null)
  const [deletion, setDeletion] = useState<DeleteDraft | null>(null)
  const [pending, setPending] = useState<
    'save' | 'impact' | 'delete' | 'reload' | null
  >(null)
  const [impactItemID, setImpactItemID] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const items =
    wardrobe.data?.pages.flatMap(
      (page) => page.items as API.WardrobeItemResponse[],
    ) ?? []

  useEffect(() => {
    if (
      (currentUser.isError && accountStatus(currentUser.error) === 401) ||
      (wardrobe.isError && accountStatus(wardrobe.error) === 401)
    ) {
      accountActions.clearCurrent()
      router.replace('/login')
    }
  }, [
    currentUser.error,
    currentUser.isError,
    wardrobe.error,
    wardrobe.isError,
    accountActions,
    router,
  ])

  function cancelEdit() {
    setEditing(null)
    setForm(emptyWardrobeForm())
    setNameError(null)
    setError(null)
    createID.current = null
  }

  function selectEdit(item: API.WardrobeItemResponse) {
    setEditing(item)
    setForm(wardrobeFormFromItem(item))
    setNameError(null)
    setError(null)
    setNotice(null)
  }

  function showActionError(cause: unknown) {
    if (accountStatus(cause) === 401) {
      accountActions.clearCurrent()
      router.replace('/login')
      return
    }
    setError(wardrobeErrorMessage(cause))
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending) return
    const validation = wardrobeNameError(form.name)
    setNameError(validation)
    if (validation) return
    setPending('save')
    setError(null)
    setNotice(null)
    try {
      if (editing) {
        const updated = await actions.update(editing.id, {
          expected_revision: editing.revision,
          name: form.name.trim(),
          category: form.category,
          availability: form.availability,
          attributes: form.attributes,
        })
        setEditing(updated)
        setForm(wardrobeFormFromItem(updated))
        setNotice('衣物已保存。')
      } else {
        const id = createID.current ?? crypto.randomUUID()
        createID.current = id
        await actions.create({
          id,
          name: form.name.trim(),
          category: form.category,
          availability: form.availability,
          source: 'wardrobe',
          attributes: form.attributes,
        })
        createID.current = null
        setForm(emptyWardrobeForm())
        setNotice('衣物已加入衣橱。')
      }
    } catch (cause) {
      showActionError(cause)
    } finally {
      setPending(null)
    }
  }

  async function reloadEditing() {
    if (!editing || pending) return
    setPending('reload')
    setError(null)
    try {
      const item = await actions.get(editing.id)
      setEditing(item)
      setForm(wardrobeFormFromItem(item))
      setNameError(null)
      await wardrobe.refetch()
    } catch (cause) {
      showActionError(cause)
    } finally {
      setPending(null)
    }
  }

  async function inspectDelete(item: API.WardrobeItemResponse) {
    if (pending) return
    setPending('impact')
    setImpactItemID(item.id)
    setError(null)
    setNotice(null)
    try {
      const impact = await actions.impact(item.id)
      setDeletion({ item, impact, policy: 'redact_snapshots' })
    } catch (cause) {
      showActionError(cause)
    } finally {
      setPending(null)
      setImpactItemID(null)
    }
  }

  async function remove() {
    if (!deletion || pending) return
    const draft = deletion
    setPending('delete')
    setError(null)
    try {
      await actions.remove(
        draft.item.id,
        draft.item.revision,
        draft.impact.expected_impact,
        draft.policy,
      )
      if (editing?.id === draft.item.id) cancelEdit()
      setNotice('衣物已删除。')
    } catch (cause) {
      showActionError(cause)
      await wardrobe.refetch()
    } finally {
      setDeletion(null)
      setPending(null)
    }
  }

  return (
    <PageShell>
      <header className="flex flex-col items-start gap-4">
        <Link
          href="/"
          className="text-primary underline-offset-4 hover:underline"
        >
          于是 OOTD
        </Link>
        <Badge variant="secondary">衣橱</Badge>
        <h1 className="page-title">我的衣橱</h1>
        <p className="max-w-2xl text-muted-foreground">
          记录真实拥有的衣物。这里使用账户数据核验；离线 Look 与推荐仍按 App
          计划交付。
        </p>
        <Button asChild variant="outline">
          <Link href="/account">账户资料</Link>
        </Button>
      </header>
      {currentUser.isPending ||
      (currentUser.isError && accountStatus(currentUser.error) === 401) ? (
        <p
          role="status"
          className="flex items-center gap-3 text-muted-foreground"
        >
          <Spinner />
          正在检查会话…
        </p>
      ) : currentUser.isError ? (
        <Alert>
          <AlertTitle>账户暂时无法读取</AlertTitle>
          <AlertDescription className="flex flex-col items-start gap-3">
            {wardrobeErrorMessage(currentUser.error)}
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
        <>
          {notice && (
            <Alert role="status">
              <AlertTitle>已完成</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          )}
          {error && (
            <Alert variant="destructive" role="alert">
              <AlertTitle>操作未完成</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <div className="grid gap-6 min-[900px]:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)] min-[900px]:items-start">
            <WardrobeEditor
              form={form}
              onChange={setForm}
              onSubmit={(event) => void save(event)}
              editing={!!editing}
              pending={pending !== null}
              nameError={nameError}
              onCancel={cancelEdit}
              onReload={() => void reloadEditing()}
            />
            <Card>
              <CardHeader>
                <CardTitle>衣物列表</CardTitle>
                <CardDescription>
                  只显示当前账户的衣物；状态和属性来自服务端。
                </CardDescription>
              </CardHeader>
              <CardContent>
                {wardrobe.isPending ||
                (wardrobe.isError && accountStatus(wardrobe.error) === 401) ? (
                  <p
                    role="status"
                    className="flex items-center gap-3 text-muted-foreground"
                  >
                    <Spinner />
                    正在读取衣橱…
                  </p>
                ) : wardrobe.isError ? (
                  <Alert>
                    <AlertTitle>衣橱暂时无法读取</AlertTitle>
                    <AlertDescription className="flex flex-col items-start gap-3">
                      {wardrobeErrorMessage(wardrobe.error)}
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => void wardrobe.refetch()}
                      >
                        重试
                      </Button>
                    </AlertDescription>
                  </Alert>
                ) : items.length === 0 ? (
                  <p className="text-muted-foreground">
                    还没有衣物。从一件常穿的开始即可。
                  </p>
                ) : (
                  <>
                    <ul className="divide-y divide-border">
                      {items.map((item) => (
                        <li
                          key={item.id}
                          className="flex flex-col gap-3 py-5 first:pt-0 last:pb-0"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <h2 className="font-medium">{item.name}</h2>
                            <Badge variant="secondary">
                              {availabilityLabels[item.availability]}
                            </Badge>
                          </div>
                          <p className="text-muted-foreground">
                            {categoryLabels[item.category]} ·{' '}
                            {item.source === 'wardrobe'
                              ? '本人衣橱'
                              : '快速记录'}{' '}
                            · 版本 {item.revision}
                          </p>
                          <div className="flex flex-wrap gap-3">
                            <Button
                              type="button"
                              variant="outline"
                              disabled={pending !== null}
                              onClick={() => selectEdit(item)}
                            >
                              编辑
                            </Button>
                            <Button
                              type="button"
                              variant="destructive"
                              disabled={pending !== null}
                              onClick={() => void inspectDelete(item)}
                            >
                              {impactItemID === item.id ? '正在检查…' : '删除'}
                            </Button>
                          </div>
                        </li>
                      ))}
                    </ul>
                    {wardrobe.hasNextPage && (
                      <Button
                        type="button"
                        variant="outline"
                        className="mt-6"
                        disabled={wardrobe.isFetchingNextPage}
                        onClick={() => void wardrobe.fetchNextPage()}
                      >
                        {wardrobe.isFetchingNextPage ? '正在加载…' : '加载更多'}
                      </Button>
                    )}
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        </>
      )}
      <AlertDialog
        open={!!deletion}
        onOpenChange={(open) => {
          if (!open) setDeletion(null)
        }}
      >
        <AlertDialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto">
          <AlertDialogHeader>
            <AlertDialogTitle>删除「{deletion?.item.name}」？</AlertDialogTitle>
            <AlertDialogDescription>
              此操作会永久删除这件衣物。服务端会再次检查版本和关联影响。
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deletion && (
            <div className="flex flex-col gap-4">
              <p>
                关联计划 {deletion.impact.affected_plan_count} 条，实际穿着{' '}
                {deletion.impact.affected_wear_event_count} 条。
              </p>
              {(deletion.impact.affected_plan_count > 0 ||
                deletion.impact.affected_wear_event_count > 0) && (
                <FieldSet>
                  <FieldLegend>如何处理关联历史</FieldLegend>
                  <Field orientation="horizontal">
                    <input
                      id="wardrobe-redact"
                      type="radio"
                      name="history-policy"
                      value="redact_snapshots"
                      checked={deletion.policy === 'redact_snapshots'}
                      onChange={() =>
                        setDeletion({ ...deletion, policy: 'redact_snapshots' })
                      }
                    />
                    <FieldLabel htmlFor="wardrobe-redact">
                      保留记录，清除这件衣物的快照
                    </FieldLabel>
                  </Field>
                  <Field orientation="horizontal">
                    <input
                      id="wardrobe-delete-history"
                      type="radio"
                      name="history-policy"
                      value="delete_affected_history"
                      checked={deletion.policy === 'delete_affected_history'}
                      onChange={() =>
                        setDeletion({
                          ...deletion,
                          policy: 'delete_affected_history',
                        })
                      }
                    />
                    <FieldLabel htmlFor="wardrobe-delete-history">
                      同时删除关联计划与实际记录
                    </FieldLabel>
                  </Field>
                </FieldSet>
              )}
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>保留衣物</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => void remove()}
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  )
}
