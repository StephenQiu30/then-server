'use client'

import { useEffect, useRef, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { WardrobeItemImage } from '@/components/wardrobe/wardrobe-item-image'
import { useWardrobePhotoUpload } from '@/hooks/wardrobe/use-wardrobe-photo'
import { garmentSamples } from '@/lib/wardrobe/samples'
import { useWearStatistics } from '@/hooks/wear-event/use-wear-statistics'
import { WearStatistics } from '@/components/wear-event/wear-statistics'
import { currentWearRange } from '@/lib/wear-event/statistics'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
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
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Field, FieldLabel, FieldLegend, FieldSet } from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { PageContainer } from '@/layout/page-container'
import { Spinner } from '@/components/ui/spinner'
import { WardrobeItemForm } from '@/components/wardrobe/wardrobe-item-form'
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

export function WardrobeView() {
  const router = useRouter()
  const accountActions = useAccountActions()
  const currentUser = useCurrentUser()
  const [category, setCategory] = useState('all')
  const [search, setSearch] = useState('')
  const [availability, setAvailability] = useState('all')
  const [editorOpen, setEditorOpen] = useState(false)
  const [range] = useState(currentWearRange)
  const enabled = !!currentUser.data
  const wardrobe = useWardrobeItems(enabled)
  const actions = useWardrobeActions()
  const photoUpload = useWardrobePhotoUpload()
  const createID = useRef<string | null>(null)
  const [form, setForm] = useState<WardrobeForm>(emptyWardrobeForm)
  const [editing, setEditing] = useState<API.WardrobeItemResponse | null>(null)
  const [deletion, setDeletion] = useState<DeleteDraft | null>(null)
  const [pending, setPending] = useState<
    'save' | 'impact' | 'delete' | 'reload' | 'photo' | null
  >(null)
  const [impactItemID, setImpactItemID] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [deletionReceipt, setDeletionReceipt] = useState<string | null>(null)
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

  useEffect(() => {
    const preview = form.photoPreview
    return () => {
      if (preview?.startsWith('blob:')) URL.revokeObjectURL(preview)
    }
  }, [form.photoPreview])

  useEffect(() => {
    const preview = form.originalPhotoPreview
    return () => {
      if (preview?.startsWith('blob:')) URL.revokeObjectURL(preview)
    }
  }, [form.originalPhotoPreview])

  function cancelEdit() {
    setEditing(null)
    setForm(emptyWardrobeForm())
    setNameError(null)
    setError(null)
    createID.current = null
  }

  function selectEdit(item: API.WardrobeItemResponse) {
    setEditorOpen(true)
    setEditing(item)
    setForm(wardrobeFormFromItem(item))
    setNameError(null)
    setError(null)
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
    try {
      let photoMediaID = form.photoMediaID
      let originalPhotoMediaID = form.originalPhotoMediaID
      if (form.photoFile) {
        const original = form.photoOriginalFile ?? form.photoFile
        originalPhotoMediaID ??= await photoUpload.upload(original)
        photoMediaID =
          form.photoFile === original
            ? originalPhotoMediaID
            : await photoUpload.upload(form.photoFile)
        setForm({ ...form, photoMediaID, originalPhotoMediaID })
      }
      if (editing) {
        const updated = await actions.update(editing.id, {
          expected_revision: editing.revision,
          name: form.name.trim(),
          sample_image: form.sampleImage,
          photo_media_id: photoMediaID,
          original_photo_media_id: originalPhotoMediaID,
          category: form.category,
          availability: form.availability,
          attributes: form.attributes,
        })
        setEditing(updated)
        setForm(wardrobeFormFromItem(updated))
        toast('衣物已保存。')
      } else {
        const id = createID.current ?? crypto.randomUUID()
        createID.current = id
        await actions.create({
          id,
          name: form.name.trim(),
          sample_image: form.sampleImage,
          photo_media_id: photoMediaID,
          original_photo_media_id: originalPhotoMediaID,
          category: form.category,
          availability: form.availability,
          source: 'wardrobe',
          attributes: form.attributes,
        })
        createID.current = null
        setForm(emptyWardrobeForm())
        toast('衣物已加入衣橱。')
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
      setDeletionReceipt('衣物已删除。')
    } catch (cause) {
      showActionError(cause)
      await wardrobe.refetch()
    } finally {
      setDeletion(null)
      setPending(null)
    }
  }

  const stats = useWearStatistics(enabled, range.from, range.to)
  const uses = new Map(
    (stats.data?.item_uses ?? []).map((entry: API.WearItemUseResponse) => [
      entry.item_id,
      entry.count,
    ]),
  )
  const filtered = items.filter(
    (item) =>
      (category === 'all' || item.category === category) &&
      (availability === 'all' || item.availability === availability) &&
      item.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()),
  )

  async function addSamples() {
    if (pending || wardrobe.isPending || wardrobe.hasNextPage) return
    setPending('save')
    setError(null)
    try {
      for (const sample of garmentSamples) {
        if (!items.some((item) => item.id === sample.id)) {
          await actions.create({
            id: sample.id,
            name: `${sample.name}（示例）`,
            category: sample.category,
            availability: 'wearable',
            source: 'wardrobe',
            attributes: {},
            sample_image: sample.key,
          })
        }
      }
      toast('五件示例衣物已加入，尚未记录实穿。')
    } catch (cause) {
      showActionError(cause)
    } finally {
      setPending(null)
    }
  }

  return (
    <PageContainer className="gap-6">
      <header className="flex flex-col items-start gap-3 pb-2">
        <h1 className="page-title">我的衣橱</h1>
        <p className="max-w-2xl text-muted-foreground">
          看清每件单品，把常穿的搭配留下来。
        </p>
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
          <div className="flex flex-wrap items-center gap-3">
            <Button asChild>
              <Link href="/plans">去搭配一套</Link>
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => setEditorOpen(!editorOpen)}
            >
              {editorOpen ? '收起添加 / 编辑' : '添加衣物'}
            </Button>
            <Button
              type="button"
              variant="ghost"
              disabled={
                pending !== null ||
                wardrobe.isPending ||
                wardrobe.isError ||
                wardrobe.hasNextPage ||
                garmentSamples.every((sample) =>
                  items.some((item) => item.id === sample.id),
                )
              }
              onClick={() => void addSamples()}
            >
              添加五件示例衣物
            </Button>
          </div>
          <div
            className={
              editorOpen
                ? 'grid gap-6 min-[1024px]:grid-cols-[320px_minmax(0,1fr)] min-[1024px]:items-start'
                : 'grid gap-6'
            }
          >
            {editorOpen && (
              <WardrobeItemForm
                key={editing?.id ?? 'new'}
                form={form}
                onChange={setForm}
                onPhotoChange={(patch) =>
                  setForm((current) => ({ ...current, ...patch }))
                }
                onPhotoBusy={(busy) => setPending(busy ? 'photo' : null)}
                onSubmit={(event) => void save(event)}
                editing={!!editing}
                pending={pending !== null}
                processing={pending === 'photo'}
                nameError={nameError}
                onCancel={cancelEdit}
                onReload={() => void reloadEditing()}
              />
            )}
            <Card>
              <CardHeader>
                <CardTitle>每件衣物，都看得见</CardTitle>
                <CardDescription>
                  筛选当前已加载的单品。每件衣物的次数来自今年已确认的实穿。
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="mb-6 flex flex-col gap-4">
                  <div
                    role="group"
                    aria-label="衣橱分类"
                    className="flex flex-wrap gap-2"
                  >
                    {Object.entries({ all: '全部', ...categoryLabels }).map(
                      ([value, label]) => (
                        <Button
                          key={value}
                          type="button"
                          variant={category === value ? 'default' : 'outline'}
                          aria-pressed={category === value}
                          onClick={() => setCategory(value)}
                        >
                          {label}
                        </Button>
                      ),
                    )}
                  </div>
                  <div className="flex flex-col gap-3 min-[641px]:flex-row">
                    <Input
                      aria-label="搜索衣物"
                      placeholder="搜索衣物名称"
                      value={search}
                      onChange={(event) => setSearch(event.target.value)}
                    />
                    <NativeSelect
                      aria-label="筛选衣物状态"
                      value={availability}
                      onChange={(event) => setAvailability(event.target.value)}
                    >
                      <NativeSelectOption value="all">
                        全部状态
                      </NativeSelectOption>
                      {Object.entries(availabilityLabels).map(
                        ([value, label]) => (
                          <NativeSelectOption key={value} value={value}>
                            {label}
                          </NativeSelectOption>
                        ),
                      )}
                    </NativeSelect>
                  </div>
                  <p role="status" className="text-sm text-muted-foreground">
                    显示 {filtered.length} / 已加载 {items.length} 件
                    {wardrobe.hasNextPage ? ' · 还有更多单品可加载' : ''}
                  </p>
                </div>
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
                  <Empty>
                    <EmptyHeader>
                      <EmptyTitle>衣橱还是空的</EmptyTitle>
                      <EmptyDescription>
                        从一件常穿的衣物开始添加。
                      </EmptyDescription>
                    </EmptyHeader>
                  </Empty>
                ) : (
                  <>
                    {filtered.length === 0 && (
                      <p className="py-8 text-center text-muted-foreground">
                        没有符合筛选条件的衣物，试试其他分类或名称。
                      </p>
                    )}
                    <ul
                      className={`grid grid-cols-2 gap-4 min-[768px]:grid-cols-3 ${editorOpen ? '' : 'min-[1200px]:grid-cols-5'}`}
                    >
                      {filtered.map((item, index) => (
                        <li
                          key={item.id}
                          className="flex min-w-0 flex-col gap-3 rounded-xl border border-border p-3"
                        >
                          <WardrobeItemImage item={item} eager={index < 5} />
                          <div className="flex flex-1 flex-col gap-2">
                            <h2 className="font-medium break-words">
                              {item.name}
                            </h2>
                            <p className="text-sm text-muted-foreground">
                              {categoryLabels[item.category]} ·{' '}
                              {availabilityLabels[item.availability]}
                            </p>
                            <p className="text-sm tabular-nums">
                              今年实穿{' '}
                              {stats.isPending
                                ? '读取中'
                                : stats.isError
                                  ? '暂不可用'
                                  : `${uses.get(item.id) ?? 0} 次`}
                            </p>
                            {item.sample_image && !item.photo_media_id && (
                              <span className="text-xs text-muted-foreground">
                                Codex 示例图
                              </span>
                            )}
                          </div>
                          <div className="flex flex-wrap gap-2">
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
                              variant="ghost"
                              disabled={pending !== null}
                              onClick={() => void inspectDelete(item)}
                            >
                              {impactItemID === item.id ? '检查中…' : '删除'}
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
          <WearStatistics enabled={enabled} />
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
                  <RadioGroup
                    value={deletion.policy}
                    onValueChange={(value) => {
                      if (
                        value === 'redact_snapshots' ||
                        value === 'delete_affected_history'
                      ) {
                        setDeletion({ ...deletion, policy: value })
                      }
                    }}
                  >
                    <Field orientation="horizontal" className="min-h-11">
                      <RadioGroupItem
                        id="wardrobe-redact"
                        value="redact_snapshots"
                      />
                      <FieldLabel
                        htmlFor="wardrobe-redact"
                        className="min-h-11 cursor-pointer"
                      >
                        保留记录，清除这件衣物的快照
                      </FieldLabel>
                    </Field>
                    <Field orientation="horizontal" className="min-h-11">
                      <RadioGroupItem
                        id="wardrobe-delete-history"
                        value="delete_affected_history"
                      />
                      <FieldLabel
                        htmlFor="wardrobe-delete-history"
                        className="min-h-11 cursor-pointer"
                      >
                        同时删除关联计划与实际记录
                      </FieldLabel>
                    </Field>
                  </RadioGroup>
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
    </PageContainer>
  )
}
