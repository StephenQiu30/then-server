'use client'

import { useEffect, useRef, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { PageShell } from '@/components/layout/page-shell'
import { Spinner } from '@/components/ui/spinner'
import { WearEditor } from '@/components/wear-event/wear-editor'
import { useAccountActions } from '@/hooks/account/use-account-actions'
import { useCurrentUser } from '@/hooks/account/use-current-user'
import { useOutfitPlans } from '@/hooks/outfit-plan/use-outfit-plans'
import { useWardrobeItems } from '@/hooks/wardrobe/use-wardrobe'
import {
  useWearEventActions,
  useWearEvents,
} from '@/hooks/wear-event/use-wear-events'
import { accountStatus } from '@/lib/account/errors'
import { duplicateCandidates, wearErrorMessage } from '@/lib/wear-event/errors'
import {
  emptyWearForm,
  followedPlanError,
  wearFormError,
  wearFormFromResponse,
  wearRequestFields,
  type WearForm,
} from '@/lib/wear-event/form'

const sourceLabels: Record<API.WearEventResponse['source_kind'], string> = {
  unplanned: '无计划记录',
  followed_plan: '按计划穿了',
  changed_plan: '换了几件',
  different_outfit: '穿了另一套',
}

export function WearEventPanel() {
  const router = useRouter()
  const accountActions = useAccountActions()
  const currentUser = useCurrentUser()
  const enabled = !!currentUser.data
  const wardrobe = useWardrobeItems(enabled)
  const plans = useOutfitPlans(enabled)
  const events = useWearEvents(enabled)
  const actions = useWearEventActions()
  const createID = useRef<string | null>(null)
  const [form, setForm] = useState<WearForm>({
    localDate: '',
    timeZone: '',
    completeness: 'complete',
    contextSummary: '',
    sourceKind: 'unplanned',
    sourcePlanID: '',
    sourcePlanRevision: null,
    items: [],
    confirmedUnavailableIDs: [],
    laundryItemIDs: [],
    duplicateConfirmations: [],
  })
  const [editing, setEditing] = useState<API.WearEventResponse | null>(null)
  const [duplicate, setDuplicate] = useState<
    API.WearEventCandidateResponse[] | null
  >(null)
  const [confirmDelete, setConfirmDelete] =
    useState<API.WearEventResponse | null>(null)
  const [pending, setPending] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [deletionReceipt, setDeletionReceipt] = useState<string | null>(null)
  const wardrobeItems =
    wardrobe.data?.pages.flatMap(
      (page) => page.items as API.WardrobeItemResponse[],
    ) ?? []
  const planItems =
    plans.data?.pages.flatMap(
      (page) => page.plans as API.OutfitPlanResponse[],
    ) ?? []
  const eventItems =
    events.data?.pages.flatMap(
      (page) => page.events as API.WearEventResponse[],
    ) ?? []

  useEffect(() => {
    const timer = window.setTimeout(() => setForm(emptyWearForm()), 0)
    return () => window.clearTimeout(timer)
  }, [])

  useEffect(() => {
    if (
      (currentUser.isError && accountStatus(currentUser.error) === 401) ||
      (wardrobe.isError && accountStatus(wardrobe.error) === 401) ||
      (plans.isError && accountStatus(plans.error) === 401) ||
      (events.isError && accountStatus(events.error) === 401)
    ) {
      accountActions.clearCurrent()
      router.replace('/login')
    }
  }, [
    currentUser.isError,
    currentUser.error,
    wardrobe.isError,
    wardrobe.error,
    plans.isError,
    plans.error,
    events.isError,
    events.error,
    accountActions,
    router,
  ])

  function showError(cause: unknown) {
    if (accountStatus(cause) === 401) {
      accountActions.clearCurrent()
      router.replace('/login')
      return
    }
    const candidates = duplicateCandidates(cause)
    if (candidates) {
      setDuplicate(candidates)
      setError(null)
      return
    }
    setDuplicate(null)
    setError(wearErrorMessage(cause))
  }

  function changeForm(value: WearForm) {
    setForm(value)
    setDuplicate(null)
    setFormError(null)
  }

  function cancelEdit() {
    setEditing(null)
    setForm(emptyWearForm())
    setFormError(null)
    setError(null)
    setDuplicate(null)
    createID.current = null
  }

  function selectEdit(event: API.WearEventResponse) {
    setEditing(event)
    setForm(wearFormFromResponse(event))
    setFormError(null)
    setError(null)
    setDuplicate(null)
  }

  function selectPlan(id: string) {
    const plan = planItems.find((item) => item.id === id)
    const itemByID = new Map(wardrobeItems.map((item) => [item.id, item]))
    const selected =
      plan && form.sourceKind === 'followed_plan'
        ? (plan.items as API.OutfitPlanItemResponse[]).flatMap(({ content }) =>
            content
              ? [
                  {
                    item_id: content.item_id,
                    revision:
                      itemByID.get(content.item_id)?.revision ??
                      content.item_revision,
                  },
                ]
              : [],
          )
        : form.items
    changeForm({
      ...form,
      sourcePlanID: id,
      sourcePlanRevision:
        plan?.revision ??
        (id === form.sourcePlanID ? form.sourcePlanRevision : null),
      items: selected,
      confirmedUnavailableIDs: [],
      laundryItemIDs: [],
    })
  }

  async function reloadEditing() {
    if (!editing || pending) return
    setPending(true)
    setError(null)
    setDuplicate(null)
    try {
      const event = await actions.get(editing.id)
      setEditing(event)
      setForm(wearFormFromResponse(event))
      setFormError(null)
      await events.refetch()
    } catch (cause) {
      showError(cause)
    } finally {
      setPending(false)
    }
  }

  async function saveEvent(confirmations: API.WearEventCandidateResponse[]) {
    if (pending) return
    let validation = wearFormError(form)
    const itemByID = new Map(wardrobeItems.map((item) => [item.id, item]))
    if (
      !validation &&
      form.items.some(
        ({ item_id, revision }) => itemByID.get(item_id)?.revision !== revision,
      )
    ) {
      validation = '衣物未载入或版本已更新，请重新选择。'
    }
    if (
      !validation &&
      form.items.some(
        ({ item_id }) =>
          itemByID.get(item_id)?.availability !== 'wearable' &&
          !form.confirmedUnavailableIDs.includes(item_id),
      )
    ) {
      validation = '请逐件确认当前不可穿但确实穿过的衣物。'
    }
    if (
      !validation &&
      form.sourceKind !== 'unplanned' &&
      !planItems.some((plan) => plan.id === form.sourcePlanID) &&
      !editing
    ) {
      validation = '请加载来源计划后再保存。'
    }
    if (!validation) {
      validation = followedPlanError(
        form,
        planItems.find((plan) => plan.id === form.sourcePlanID),
      )
    }
    setFormError(validation)
    if (validation) return
    setPending(true)
    setError(null)
    setDuplicate(null)
    try {
      const fields = wearRequestFields(form)
      fields.confirmed_unavailable_ids =
        fields.confirmed_unavailable_ids.filter(
          (id) => itemByID.get(id)?.availability !== 'wearable',
        )
      fields.duplicate_confirmations = confirmations
      if (editing) {
        const event = await actions.update(editing.id, {
          ...fields,
          expected_revision: editing.revision,
        })
        setEditing(event)
        setForm(wearFormFromResponse(event))
        toast('实际穿着已纠正', {
          description: '关联计划已按服务端事实重算。',
        })
      } else {
        const id = createID.current ?? crypto.randomUUID()
        createID.current = id
        await actions.create({ id, ...fields })
        createID.current = null
        setForm(emptyWearForm())
        toast('实际穿着已保存。')
      }
    } catch (cause) {
      showError(cause)
    } finally {
      setPending(false)
    }
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    void saveEvent([])
  }

  async function remove() {
    if (!confirmDelete || pending) return
    const event = confirmDelete
    setConfirmDelete(null)
    setPending(true)
    setError(null)
    try {
      await actions.remove(event.id, event.revision)
      if (editing?.id === event.id) cancelEdit()
      setDeletionReceipt('实际记录已删除，关联计划状态已重算。')
    } catch (cause) {
      showError(cause)
      await events.refetch()
    } finally {
      setPending(false)
    }
  }

  return (
    <PageShell>
      <header className="flex flex-col items-start gap-4">
        <Link href="/" className="text-link underline-offset-4 hover:underline">
          于是 OOTD
        </Link>
        <Badge variant="secondary">实际</Badge>
        <h1 className="page-title">我的实际穿着</h1>
        <p className="max-w-2xl text-muted-foreground">
          只记录你明确确认穿过的衣物；计划本身不会增加实际记录。
        </p>
        <div className="flex flex-wrap gap-3">
          <Button asChild variant="outline">
            <Link href="/plans">穿搭计划</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/wardrobe">管理衣橱</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/account">账户资料</Link>
          </Button>
        </div>
      </header>
      {currentUser.isPending ||
      (currentUser.isError && accountStatus(currentUser.error) === 401) ? (
        <p
          role="status"
          className="flex items-center gap-3 text-muted-foreground"
        >
          <Spinner /> 正在检查会话…
        </p>
      ) : currentUser.isError ? (
        <Alert>
          <AlertTitle>账户暂时无法读取</AlertTitle>
          <AlertDescription className="flex flex-col items-start gap-3">
            {wearErrorMessage(currentUser.error)}
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
          {deletionReceipt && (
            <Alert role="status">
              <AlertTitle>删除完成</AlertTitle>
              <AlertDescription>{deletionReceipt}</AlertDescription>
            </Alert>
          )}
          {error && (
            <Alert variant="destructive" role="alert">
              <AlertTitle>操作未完成</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          {duplicate && (
            <Alert role="alert">
              <AlertTitle>同日有相似的实际记录</AlertTitle>
              <AlertDescription className="flex flex-col items-start gap-3">
                <p>
                  找到 {duplicate.length}{' '}
                  条相似记录。请核对列表；确实再次穿了这套时，再主动确认保存。若候选已变化，服务端会重新要求确认。
                </p>
                <ul className="list-inside list-disc">
                  {duplicate.map((item) => (
                    <li key={item.id}>
                      {item.id.slice(0, 8)} · 版本 {item.revision}
                    </li>
                  ))}
                </ul>
                <div className="flex flex-wrap gap-3">
                  <Button
                    type="button"
                    disabled={pending}
                    onClick={() => void saveEvent(duplicate)}
                  >
                    确认仍要保存
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    disabled={pending}
                    onClick={() => setDuplicate(null)}
                  >
                    返回修改
                  </Button>
                </div>
              </AlertDescription>
            </Alert>
          )}
          <div className="grid gap-6 min-[900px]:grid-cols-2 min-[900px]:items-start">
            <WearEditor
              form={form}
              onChange={changeForm}
              onSubmit={submit}
              editing={!!editing}
              pending={pending || !form.timeZone}
              error={formError}
              items={wardrobeItems}
              itemsLoading={wardrobe.isPending || wardrobe.isFetchingNextPage}
              hasMoreItems={!!wardrobe.hasNextPage}
              loadMoreItems={() => void wardrobe.fetchNextPage()}
              plans={planItems}
              plansLoading={plans.isPending || plans.isFetchingNextPage}
              hasMorePlans={!!plans.hasNextPage}
              loadMorePlans={() => void plans.fetchNextPage()}
              onSelectPlan={selectPlan}
              onCancel={cancelEdit}
              onReload={() => void reloadEditing()}
            />
            <Card>
              <CardHeader>
                <CardTitle>实际记录列表</CardTitle>
                <CardDescription>
                  按原本地日期回看；未知衣物保持未知。
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-4">
                {wardrobe.isError && accountStatus(wardrobe.error) !== 401 && (
                  <Alert>
                    <AlertTitle>衣橱暂时无法读取</AlertTitle>
                    <AlertDescription className="flex flex-col items-start gap-3">
                      {wearErrorMessage(wardrobe.error)}
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => void wardrobe.refetch()}
                      >
                        重试
                      </Button>
                    </AlertDescription>
                  </Alert>
                )}
                {plans.isError && accountStatus(plans.error) !== 401 && (
                  <Alert>
                    <AlertTitle>计划暂时无法读取</AlertTitle>
                    <AlertDescription className="flex flex-col items-start gap-3">
                      {wearErrorMessage(plans.error)}
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => void plans.refetch()}
                      >
                        重试
                      </Button>
                    </AlertDescription>
                  </Alert>
                )}
                {events.isPending ||
                (events.isError && accountStatus(events.error) === 401) ? (
                  <p
                    role="status"
                    className="flex items-center gap-2 text-muted-foreground"
                  >
                    <Spinner /> 正在读取实际记录…
                  </p>
                ) : events.isError ? (
                  <Alert>
                    <AlertTitle>实际记录暂时无法读取</AlertTitle>
                    <AlertDescription className="flex flex-col items-start gap-3">
                      {wearErrorMessage(events.error)}
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => void events.refetch()}
                      >
                        重试
                      </Button>
                    </AlertDescription>
                  </Alert>
                ) : eventItems.length === 0 ? (
                  <p className="text-muted-foreground">
                    还没有实际记录。穿过之后可主动记下。
                  </p>
                ) : (
                  <>
                    <ul className="divide-y divide-border">
                      {eventItems.map((event) => (
                        <li
                          key={event.id}
                          className="flex flex-col gap-3 py-5 first:pt-0 last:pb-0"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <h2 className="font-medium">{event.local_date}</h2>
                            <Badge variant="secondary">
                              {event.completeness === 'partial'
                                ? '部分记录'
                                : '完整记录'}
                            </Badge>
                          </div>
                          <p className="text-muted-foreground">
                            {sourceLabels[event.source_kind]} ·{' '}
                            {event.time_zone} · 版本 {event.revision}
                          </p>
                          {event.context_summary && (
                            <p>{event.context_summary}</p>
                          )}
                          {event.source_plan_id && (
                            <p className="break-all text-muted-foreground">
                              关联计划 {event.source_plan_id}
                            </p>
                          )}
                          <ul className="list-inside list-disc text-muted-foreground">
                            {(event.items as API.OutfitPlanItemResponse[]).map(
                              (item) => (
                                <li key={item.ordinal}>
                                  {item.content?.name ?? '已清除的衣物快照'}
                                </li>
                              ),
                            )}
                          </ul>
                          {(event.items as API.OutfitPlanItemResponse[]).some(
                            (item) => !item.content,
                          ) && (
                            <p className="text-muted-foreground">
                              含已清除的衣物快照；可保留历史或另建记录。
                            </p>
                          )}
                          <div className="flex flex-wrap gap-3">
                            <Button
                              type="button"
                              variant="outline"
                              disabled={
                                pending ||
                                (
                                  event.items as API.OutfitPlanItemResponse[]
                                ).some((item) => !item.content)
                              }
                              onClick={() => selectEdit(event)}
                            >
                              纠正
                            </Button>
                            <Button
                              type="button"
                              variant="destructive"
                              disabled={pending}
                              onClick={() => setConfirmDelete(event)}
                            >
                              删除
                            </Button>
                          </div>
                        </li>
                      ))}
                    </ul>
                    {events.hasNextPage && (
                      <Button
                        type="button"
                        variant="outline"
                        disabled={events.isFetchingNextPage}
                        onClick={() => void events.fetchNextPage()}
                      >
                        {events.isFetchingNextPage
                          ? '正在加载…'
                          : '加载更多记录'}
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
        open={!!confirmDelete}
        onOpenChange={(open) => {
          if (!open) setConfirmDelete(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>永久删除这条实际记录？</AlertDialogTitle>
            <AlertDialogDescription>
              删除后关联计划的状态会按剩余有效实际记录重算。此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>保留记录</AlertDialogCancel>
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
