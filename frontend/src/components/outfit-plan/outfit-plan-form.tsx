'use client'

import type { FormEvent } from 'react'
import { OutfitFlatlayEditor } from '@/components/outfit-plan/outfit-flatlay-editor'
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
import { Spinner } from '@/components/ui/spinner'
import { availabilityLabels } from '@/lib/wardrobe/form'
import type { PlanForm } from '@/lib/outfit-plan/form'

export function OutfitPlanForm({
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
  onCancel,
  onReload,
}: {
  form: PlanForm
  onChange: (form: PlanForm) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  editing: boolean
  pending: boolean
  error: string | null
  items: API.WardrobeItemResponse[]
  itemsLoading: boolean
  hasMoreItems: boolean
  loadMoreItems: () => void
  onCancel: () => void
  onReload: () => void
}) {
  const loadedIDs = new Set(items.map((item) => item.id))
  const missing = form.items.filter((item) => !loadedIDs.has(item.item_id))

  return (
    <Card>
      <CardHeader>
        <CardTitle>{editing ? '调整这套搭配' : '搭配一套，留给今天'}</CardTitle>
        <CardDescription>
          计划只记录准备穿什么，不会自动产生实际穿着。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} noValidate className="flex flex-col gap-6">
          <FieldGroup className="grid min-[641px]:grid-cols-2">
            <Field>
              <FieldLabel htmlFor="plan-date">本地日期</FieldLabel>
              <Input
                id="plan-date"
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
              <FieldLabel htmlFor="plan-context">场景说明（可选）</FieldLabel>
              <Input
                id="plan-context"
                value={form.contextSummary}
                disabled={pending}
                onChange={(event) =>
                  onChange({ ...form, contextSummary: event.target.value })
                }
              />
            </Field>
          </FieldGroup>
          <FieldSet className="gap-3">
            <FieldLegend>选择真实衣物</FieldLegend>
            <p className="text-muted-foreground">
              每类点选一件即可替换，其他类别保持原样。
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
              <OutfitFlatlayEditor
                form={form}
                onChange={onChange}
                items={items}
                pending={pending}
              />
            )}
            {items
              .filter(
                (item) =>
                  item.availability !== 'wearable' &&
                  form.items.some((entry) => entry.item_id === item.id),
              )
              .map((item) => (
                <Field
                  key={item.id}
                  orientation="horizontal"
                  className="min-h-11"
                >
                  <Checkbox
                    id={`plan-confirm-unavailable-${item.id}`}
                    checked={form.confirmedUnavailableIDs.includes(item.id)}
                    disabled={pending}
                    onCheckedChange={(checked) =>
                      onChange({
                        ...form,
                        confirmedUnavailableIDs:
                          checked === true
                            ? [...form.confirmedUnavailableIDs, item.id]
                            : form.confirmedUnavailableIDs.filter(
                                (id) => id !== item.id,
                              ),
                      })
                    }
                  />
                  <FieldLabel
                    htmlFor={`plan-confirm-unavailable-${item.id}`}
                    className="min-h-11 cursor-pointer"
                  >
                    {item.name}当前{availabilityLabels[item.availability]}
                    ，仍加入计划
                  </FieldLabel>
                </Field>
              ))}
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
                  有 {missing.length}{' '}
                  件已选衣物未在当前列表中，加载更多或移除后再保存。
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
            <Button type="submit" disabled={pending || itemsLoading}>
              {pending && <Spinner data-icon="inline-start" />}
              {pending ? '正在保存…' : editing ? '保存修改' : '保存计划'}
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
                  重新加载计划
                </Button>
              </>
            )}
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
