'use client'

import type { FormEvent } from 'react'
import { WardrobeItemImage } from '@/components/wardrobe/wardrobe-item-image'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import { availabilityLabels } from '@/lib/wardrobe/form'
import type { WearForm } from '@/lib/wear-event/form'

const sourceLabels: Record<WearForm['sourceKind'], string> = {
  unplanned: '无计划直接记录',
  followed_plan: '按计划穿了',
  changed_plan: '计划中换了衣物',
  different_outfit: '穿了另一套',
}

export function WearEventForm({
  form,
  onChange,
  onSubmit,
  editing,
  pending,
  error,
  items,
  itemsLoading,
  hasMoreItems,
  loadMoreItems,
  plans,
  plansLoading,
  hasMorePlans,
  loadMorePlans,
  onSelectPlan,
  onCancel,
  onReload,
}: {
  form: WearForm
  onChange: (value: WearForm) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  editing: boolean
  pending: boolean
  error: string | null
  items: API.WardrobeItemResponse[]
  itemsLoading: boolean
  hasMoreItems: boolean
  loadMoreItems: () => void
  plans: API.OutfitPlanResponse[]
  plansLoading: boolean
  hasMorePlans: boolean
  loadMorePlans: () => void
  onSelectPlan: (id: string) => void
  onCancel: () => void
  onReload: () => void
}) {
  const loadedIDs = new Set(items.map((item) => item.id))
  const missing = form.items.filter((item) => !loadedIDs.has(item.item_id))
  const selectedPlanLoaded = plans.some((plan) => plan.id === form.sourcePlanID)

  function toggleItem(item: API.WardrobeItemResponse, checked: boolean) {
    onChange({
      ...form,
      items: checked
        ? [...form.items, { item_id: item.id, revision: item.revision }]
        : form.items.filter((selection) => selection.item_id !== item.id),
      confirmedUnavailableIDs: form.confirmedUnavailableIDs.filter(
        (id) => id !== item.id,
      ),
      laundryItemIDs: form.laundryItemIDs.filter((id) => id !== item.id),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{editing ? '纠正实际记录' : '记录实际穿着'}</CardTitle>
        <CardDescription>
          只有主动保存才形成实际穿着；未知衣物可标为部分记录。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} noValidate className="flex flex-col gap-6">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="wear-date">实际日期</FieldLabel>
              <Input
                id="wear-date"
                type="date"
                required
                disabled={pending}
                value={form.localDate}
                onChange={(event) =>
                  onChange({ ...form, localDate: event.target.value })
                }
              />
              <FieldDescription>原始时区：{form.timeZone}</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="wear-completeness">记录范围</FieldLabel>
              <NativeSelect
                id="wear-completeness"
                value={form.completeness}
                disabled={pending}
                onChange={(event) =>
                  onChange({
                    ...form,
                    completeness: event.target
                      .value as WearForm['completeness'],
                  })
                }
              >
                <NativeSelectOption value="complete">
                  完整记录
                </NativeSelectOption>
                <NativeSelectOption value="partial">
                  部分记录，其他衣物未知
                </NativeSelectOption>
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="wear-source">与计划的关系</FieldLabel>
              <NativeSelect
                id="wear-source"
                value={form.sourceKind}
                disabled={pending}
                onChange={(event) =>
                  onChange({
                    ...form,
                    sourceKind: event.target.value as WearForm['sourceKind'],
                    sourcePlanID:
                      event.target.value === 'unplanned'
                        ? ''
                        : form.sourcePlanID,
                    sourcePlanRevision:
                      event.target.value === 'unplanned'
                        ? null
                        : form.sourcePlanRevision,
                  })
                }
              >
                {Object.entries(sourceLabels).map(([value, label]) => (
                  <NativeSelectOption key={value} value={value}>
                    {label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Field>
            {form.sourceKind !== 'unplanned' && (
              <Field>
                <FieldLabel htmlFor="wear-plan">来源计划</FieldLabel>
                <NativeSelect
                  id="wear-plan"
                  value={form.sourcePlanID}
                  disabled={pending || plansLoading}
                  onChange={(event) => onSelectPlan(event.target.value)}
                >
                  <NativeSelectOption value="">选择计划</NativeSelectOption>
                  {form.sourcePlanID && !selectedPlanLoaded && (
                    <NativeSelectOption value={form.sourcePlanID}>
                      原关联计划 · {form.sourcePlanID.slice(0, 8)}
                    </NativeSelectOption>
                  )}
                  {plans
                    .filter(
                      (plan) =>
                        plan.status === 'active' ||
                        plan.status === 'completed' ||
                        plan.id === form.sourcePlanID,
                    )
                    .map((plan) => (
                      <NativeSelectOption key={plan.id} value={plan.id}>
                        {plan.local_date} ·{' '}
                        {plan.context_summary || plan.id.slice(0, 8)} · 版本{' '}
                        {plan.revision}
                      </NativeSelectOption>
                    ))}
                </NativeSelect>
                {hasMorePlans && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={pending || plansLoading}
                    onClick={loadMorePlans}
                  >
                    加载更多计划
                  </Button>
                )}
                <FieldDescription>
                  选择按计划穿了时，会带入计划衣物；保存前仍需核对当前状态。
                </FieldDescription>
              </Field>
            )}
            <Field>
              <FieldLabel htmlFor="wear-context">场景说明（可选）</FieldLabel>
              <Input
                id="wear-context"
                value={form.contextSummary}
                disabled={pending}
                onChange={(event) =>
                  onChange({ ...form, contextSummary: event.target.value })
                }
              />
            </Field>
          </FieldGroup>
          <FieldSet className="gap-3">
            <FieldLegend>实际穿过的衣物</FieldLegend>
            <p className="text-muted-foreground">
              按选中顺序保存，最多 20 件；待洗不会自动勾选。
            </p>
            {itemsLoading ? (
              <p
                role="status"
                className="flex items-center gap-2 text-muted-foreground"
              >
                <Spinner /> 正在读取衣橱…
              </p>
            ) : items.length === 0 ? (
              <Empty>
                <EmptyHeader>
                  <EmptyTitle>衣橱还是空的</EmptyTitle>
                  <EmptyDescription>请先添加真实衣物。</EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className="divide-y divide-border rounded-2xl border border-border px-4">
                {items.map((item) => {
                  const selected = form.items.some(
                    (selection) => selection.item_id === item.id,
                  )
                  const unavailable = item.availability !== 'wearable'
                  const itemDisabled =
                    pending || (!selected && form.items.length >= 20)
                  return (
                    <div key={item.id} className="py-2">
                      <Field
                        orientation="horizontal"
                        className="min-h-11"
                        data-disabled={itemDisabled || undefined}
                      >
                        <Checkbox
                          id={`wear-item-${item.id}`}
                          checked={selected}
                          disabled={itemDisabled}
                          onCheckedChange={(checked) =>
                            toggleItem(item, checked === true)
                          }
                        />
                        <FieldLabel
                          htmlFor={`wear-item-${item.id}`}
                          className="min-h-11 min-w-0 cursor-pointer break-words"
                        >
                          <WardrobeItemImage
                            item={item}
                            decorative
                            className="size-14 shrink-0"
                          />
                          <span>
                            {item.name} ·{' '}
                            {availabilityLabels[item.availability]}
                          </span>
                        </FieldLabel>
                      </Field>
                      {selected && (
                        <div className="ml-8 flex flex-col gap-1">
                          {unavailable && (
                            <Field
                              orientation="horizontal"
                              className="min-h-11 text-muted-foreground"
                              data-disabled={pending || undefined}
                            >
                              <Checkbox
                                id={`wear-confirm-unavailable-${item.id}`}
                                checked={form.confirmedUnavailableIDs.includes(
                                  item.id,
                                )}
                                disabled={pending}
                                onCheckedChange={(checked) =>
                                  onChange({
                                    ...form,
                                    confirmedUnavailableIDs:
                                      checked === true
                                        ? [
                                            ...form.confirmedUnavailableIDs,
                                            item.id,
                                          ]
                                        : form.confirmedUnavailableIDs.filter(
                                            (id) => id !== item.id,
                                          ),
                                  })
                                }
                              />
                              <FieldLabel
                                htmlFor={`wear-confirm-unavailable-${item.id}`}
                                className="min-h-11 cursor-pointer"
                              >
                                我知道这件衣物当前不可穿，确实穿了
                              </FieldLabel>
                            </Field>
                          )}
                          <Field
                            orientation="horizontal"
                            className="min-h-11 text-muted-foreground"
                            data-disabled={pending || undefined}
                          >
                            <Checkbox
                              id={`wear-laundry-${item.id}`}
                              checked={form.laundryItemIDs.includes(item.id)}
                              disabled={pending}
                              onCheckedChange={(checked) =>
                                onChange({
                                  ...form,
                                  laundryItemIDs:
                                    checked === true
                                      ? [...form.laundryItemIDs, item.id]
                                      : form.laundryItemIDs.filter(
                                          (id) => id !== item.id,
                                        ),
                                })
                              }
                            />
                            <FieldLabel
                              htmlFor={`wear-laundry-${item.id}`}
                              className="min-h-11 cursor-pointer"
                            >
                              保存时把这件衣物标为待洗
                            </FieldLabel>
                          </Field>
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
            {hasMoreItems && (
              <Button
                type="button"
                variant="outline"
                disabled={pending || itemsLoading}
                onClick={loadMoreItems}
              >
                加载更多衣物
              </Button>
            )}
            {missing.length > 0 && (
              <div className="flex flex-col gap-2 text-muted-foreground">
                <p>
                  {missing.length} 件已选衣物尚未载入。加载更多或移除后再保存。
                </p>
                {missing.map((selection) => (
                  <Button
                    key={selection.item_id}
                    type="button"
                    variant="outline"
                    disabled={pending}
                    onClick={() =>
                      onChange({
                        ...form,
                        items: form.items.filter(
                          (item) => item.item_id !== selection.item_id,
                        ),
                      })
                    }
                  >
                    移除未加载衣物 {selection.item_id.slice(0, 8)}
                  </Button>
                ))}
              </div>
            )}
          </FieldSet>
          {error && (
            <Field data-invalid>
              <FieldError role="alert">{error}</FieldError>
            </Field>
          )}
          <div className="flex flex-wrap gap-3">
            <Button
              type="submit"
              disabled={pending || itemsLoading || plansLoading}
            >
              {pending && <Spinner data-icon="inline-start" />}
              {pending ? '正在保存…' : editing ? '保存纠正' : '确认实际穿了'}
            </Button>
            {editing && (
              <>
                <Button
                  type="button"
                  variant="outline"
                  disabled={pending}
                  onClick={onCancel}
                >
                  取消编辑
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  disabled={pending}
                  onClick={onReload}
                >
                  重新加载记录
                </Button>
              </>
            )}
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
