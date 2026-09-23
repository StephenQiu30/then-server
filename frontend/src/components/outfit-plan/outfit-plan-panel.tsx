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
import { PageShell } from '@/components/layout/page-shell'
import { Spinner } from '@/components/ui/spinner'
import { PlanEditor } from '@/components/outfit-plan/plan-editor'
import { useAccountActions } from '@/hooks/account/use-account-actions'
import { useCurrentUser } from '@/hooks/account/use-current-user'
import {
  useOutfitPlanActions,
  useOutfitPlans,
} from '@/hooks/outfit-plan/use-outfit-plans'
import { useWardrobeItems } from '@/hooks/wardrobe/use-wardrobe'
import { accountStatus } from '@/lib/account/errors'
import { planErrorMessage } from '@/lib/outfit-plan/errors'
import {
  emptyPlanForm,
  todayInTimeZone,
  planFormError,
  planFormFromResponse,
  planRequestFields,
  type PlanForm,
} from '@/lib/outfit-plan/form'

type PlanAction = 'cancel' | 'not_worn' | 'delete'
type ActionDraft = { plan: API.OutfitPlanResponse; action: PlanAction }

const statusLabels: Record<API.OutfitPlanResponse['status'], string> = {
  active: '待确认',
  completed: '已穿',
  not_worn: '未穿',
  cancelled: '已取消',
}

const actionLabels: Record<PlanAction, string> = {
  cancel: '取消计划',
  not_worn: '确认未穿',
  delete: '永久删除',
}

export function OutfitPlanPanel() {
  const router = useRouter()
  const accountActions = useAccountActions()
  const currentUser = useCurrentUser()
  const enabled = !!currentUser.data
  const wardrobe = useWardrobeItems(enabled)
  const plans = useOutfitPlans(enabled)
  const actions = useOutfitPlanActions()
  const createID = useRef<string | null>(null)
  const [form, setForm] = useState<PlanForm>({
    localDate: '',
    timeZone: '',
    contextSummary: '',
    items: [],
    confirmedUnavailableIDs: [],
  })
  const [editing, setEditing] = useState<API.OutfitPlanResponse | null>(null)
  const [confirmation, setConfirmation] = useState<ActionDraft | null>(null)
  const [pending, setPending] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const wardrobeItems =
    wardrobe.data?.pages.flatMap(
      (page) => page.items as API.WardrobeItemResponse[],
    ) ?? []
  const planItems =
    plans.data?.pages.flatMap(
      (page) => page.plans as API.OutfitPlanResponse[],
    ) ?? []

  useEffect(() => {
    const timer = window.setTimeout(() => setForm(emptyPlanForm()), 0)
    return () => window.clearTimeout(timer)
  }, [])

  useEffect(() => {
    if (
      (currentUser.isError && accountStatus(currentUser.error) === 401) ||
      (wardrobe.isError && accountStatus(wardrobe.error) === 401) ||
      (plans.isError && accountStatus(plans.error) === 401)
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
    accountActions,
    router,
  ])

  function showError(cause: unknown) {
    if (accountStatus(cause) === 401) {
      accountActions.clearCurrent()
      router.replace('/login')
      return
    }
    setError(planErrorMessage(cause))
  }

  function cancelEdit() {
    setEditing(null)
    setForm(emptyPlanForm())
    setFormError(null)
    setError(null)
    createID.current = null
  }

  function selectEdit(plan: API.OutfitPlanResponse) {
    setEditing(plan)
    setForm(planFormFromResponse(plan))
    setFormError(null)
    setError(null)
    setNotice(null)
  }

  async function reloadEditing() {
    if (!editing || pending) return
    setPending(true)
    setError(null)
    try {
      const plan = await actions.get(editing.id)
      setEditing(plan)
      setForm(planFormFromResponse(plan))
      setFormError(null)
      await plans.refetch()
    } catch (cause) {
      showError(cause)
    } finally {
      setPending(false)
    }
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending) return
    let validation = planFormError(form, editing?.local_date)
    const itemByID = new Map(wardrobeItems.map((item) => [item.id, item]))
    if (
      !validation &&
      form.items.some(({ item_id }) => !itemByID.has(item_id))
    ) {
      validation = '请加载或移除尚未显示的衣物。'
    }
    if (
      !validation &&
      form.items.some(({ item_id }) => {
        const item = itemByID.get(item_id)
        return (
          item?.availability !== 'wearable' &&
          !form.confirmedUnavailableIDs.includes(item_id)
        )
      })
    ) {
      validation = '请逐件确认仍要计划穿当前不可用的衣物。'
    }
    setFormError(validation)
    if (validation) return
    setPending(true)
    setError(null)
    setNotice(null)
    try {
      const fields = planRequestFields(form)
      fields.confirmed_unavailable_ids =
        fields.confirmed_unavailable_ids.filter(
          (id) => itemByID.get(id)?.availability !== 'wearable',
        )
      if (editing) {
        const plan = await actions.update(editing.id, {
          ...fields,
          expected_revision: editing.revision,
        })
        setEditing(plan)
        setForm(planFormFromResponse(plan))
        setNotice('计划已保存；实际穿着仍需另行确认。')
      } else {
        const id = createID.current ?? crypto.randomUUID()
        createID.current = id
        await actions.create({ id, ...fields })
        createID.current = null
        setForm(emptyPlanForm())
        setNotice('计划已保存；尚未记录实际穿着。')
      }
    } catch (cause) {
      showError(cause)
    } finally {
      setPending(false)
    }
  }

  async function runAction() {
    if (!confirmation || pending) return
    const { plan, action } = confirmation
    setConfirmation(null)
    setPending(true)
    setError(null)
    setNotice(null)
    try {
      if (action === 'cancel') await actions.cancel(plan.id, plan.revision)
      if (action === 'not_worn')
        await actions.markNotWorn(plan.id, plan.revision)
      if (action === 'delete') await actions.remove(plan.id, plan.revision)
      if (editing?.id === plan.id) cancelEdit()
      setNotice(`${actionLabels[action]}已完成。`)
    } catch (cause) {
      showError(cause)
      await plans.refetch()
    } finally {
      setPending(false)
    }
  }

  async function restore(plan: API.OutfitPlanResponse) {
    if (pending) return
    setPending(true)
    setError(null)
    try {
      await actions.restore(plan.id, plan.revision)
      setNotice('计划已恢复为待确认。')
    } catch (cause) {
      showError(cause)
      await plans.refetch()
    } finally {
      setPending(false)
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
        <Badge variant="secondary">计划</Badge>
        <h1 className="page-title">我的穿搭计划</h1>
        <p className="max-w-2xl text-muted-foreground">
          用真实衣物记录准备穿什么。计划、实际穿着和 Look 各自独立。
        </p>
        <div className="flex flex-wrap gap-3">
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
            {planErrorMessage(currentUser.error)}
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
          <div className="grid gap-6 min-[900px]:grid-cols-2 min-[900px]:items-start">
            <PlanEditor
              form={form}
              onChange={setForm}
              onSubmit={(event) => void save(event)}
              editing={!!editing}
              pending={pending || !form.timeZone}
              error={formError}
              items={wardrobeItems}
              itemsLoading={wardrobe.isPending || wardrobe.isFetchingNextPage}
              hasMoreItems={!!wardrobe.hasNextPage}
              loadMoreItems={() => void wardrobe.fetchNextPage()}
              onCancel={cancelEdit}
              onReload={() => void reloadEditing()}
            />
            <Card>
              <CardHeader>
                <CardTitle>计划列表</CardTitle>
                <CardDescription>
                  按原本地日期显示；保存计划不增加穿着次数。
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-4">
                {wardrobe.isError && accountStatus(wardrobe.error) !== 401 && (
                  <Alert>
                    <AlertTitle>衣橱暂时无法读取</AlertTitle>
                    <AlertDescription className="flex flex-col items-start gap-3">
                      {planErrorMessage(wardrobe.error)}
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
                {plans.isPending ||
                (plans.isError && accountStatus(plans.error) === 401) ? (
                  <p
                    role="status"
                    className="flex items-center gap-2 text-muted-foreground"
                  >
                    <Spinner /> 正在读取计划…
                  </p>
                ) : plans.isError ? (
                  <Alert>
                    <AlertTitle>计划暂时无法读取</AlertTitle>
                    <AlertDescription className="flex flex-col items-start gap-3">
                      {planErrorMessage(plans.error)}
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => void plans.refetch()}
                      >
                        重试
                      </Button>
                    </AlertDescription>
                  </Alert>
                ) : planItems.length === 0 ? (
                  <p className="text-muted-foreground">
                    还没有计划。从衣橱中选一套即可。
                  </p>
                ) : (
                  <>
                    <ul className="divide-y divide-border">
                      {planItems.map((plan) => (
                        <li
                          key={plan.id}
                          className="flex flex-col gap-3 py-5 first:pt-0 last:pb-0"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <h2 className="font-medium">{plan.local_date}</h2>
                            <Badge variant="secondary">
                              {statusLabels[plan.status]}
                            </Badge>
                          </div>
                          <p className="text-muted-foreground">
                            {plan.time_zone} · 版本 {plan.revision}
                          </p>
                          {plan.context_summary && (
                            <p>{plan.context_summary}</p>
                          )}
                          <ul className="list-inside list-disc text-muted-foreground">
                            {(plan.items as API.OutfitPlanItemResponse[]).map(
                              (item) => (
                                <li key={item.ordinal}>
                                  {item.content?.name ?? '已清除的衣物快照'}
                                </li>
                              ),
                            )}
                          </ul>
                          {(plan.items as API.OutfitPlanItemResponse[]).some(
                            (item) => !item.content,
                          ) && (
                            <p className="text-muted-foreground">
                              含已清除的衣物快照；可保留历史或重新创建计划。
                            </p>
                          )}
                          <div className="flex flex-wrap gap-3">
                            {plan.status === 'active' && (
                              <>
                                <Button
                                  type="button"
                                  variant="outline"
                                  disabled={
                                    pending ||
                                    (
                                      plan.items as API.OutfitPlanItemResponse[]
                                    ).some((item) => !item.content)
                                  }
                                  onClick={() => selectEdit(plan)}
                                >
                                  编辑
                                </Button>
                                <Button
                                  type="button"
                                  variant="outline"
                                  disabled={pending}
                                  onClick={() =>
                                    setConfirmation({ plan, action: 'cancel' })
                                  }
                                >
                                  取消计划
                                </Button>
                                {plan.local_date <=
                                  todayInTimeZone(plan.time_zone) && (
                                  <Button
                                    type="button"
                                    variant="outline"
                                    disabled={pending}
                                    onClick={() =>
                                      setConfirmation({
                                        plan,
                                        action: 'not_worn',
                                      })
                                    }
                                  >
                                    确认未穿
                                  </Button>
                                )}
                              </>
                            )}
                            {plan.status === 'not_worn' && (
                              <Button
                                type="button"
                                variant="outline"
                                disabled={pending}
                                onClick={() => void restore(plan)}
                              >
                                恢复计划
                              </Button>
                            )}
                            <Button
                              type="button"
                              variant="destructive"
                              disabled={pending}
                              onClick={() =>
                                setConfirmation({ plan, action: 'delete' })
                              }
                            >
                              删除
                            </Button>
                          </div>
                        </li>
                      ))}
                    </ul>
                    {plans.hasNextPage && (
                      <Button
                        type="button"
                        variant="outline"
                        disabled={plans.isFetchingNextPage}
                        onClick={() => void plans.fetchNextPage()}
                      >
                        {plans.isFetchingNextPage
                          ? '正在加载…'
                          : '加载更多计划'}
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
        open={!!confirmation}
        onOpenChange={(open) => {
          if (!open) setConfirmation(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmation
                ? `${actionLabels[confirmation.action]}？`
                : '确认操作'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirmation?.action === 'delete'
                ? '此操作会永久删除这条计划；独立的实际穿着不会随之删除。'
                : confirmation?.action === 'not_worn'
                  ? '这会把计划标为未穿，不会创建实际穿着。之后仍可恢复计划。'
                  : '取消后计划将只读，不会创建实际穿着。'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>保留计划</AlertDialogCancel>
            <AlertDialogAction
              variant={
                confirmation?.action === 'delete' ? 'destructive' : 'default'
              }
              onClick={() => void runAction()}
            >
              确认
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  )
}
