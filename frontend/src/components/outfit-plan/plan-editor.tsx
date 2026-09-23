'use client'

import type { FormEvent } from 'react'
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
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { availabilityLabels } from '@/lib/wardrobe/form'
import type { PlanForm } from '@/lib/outfit-plan/form'

export function PlanEditor({
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
  function toggleItem(item: API.WardrobeItemResponse, checked: boolean) {
    onChange({
      ...form,
      items: checked
        ? [...form.items, { item_id: item.id, revision: item.revision }]
        : form.items.filter((selection) => selection.item_id !== item.id),
      confirmedUnavailableIDs: form.confirmedUnavailableIDs.filter(
        (id) => id !== item.id,
      ),
    })
  }

  const loadedIDs = new Set(items.map((item) => item.id))
  const missing = form.items.filter((item) => !loadedIDs.has(item.item_id))

  return (
    <Card>
      <CardHeader>
        <CardTitle>{editing ? '编辑计划' : '保存一条计划'}</CardTitle>
        <CardDescription>
          计划只记录准备穿什么，不会自动产生实际穿着。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} noValidate className="flex flex-col gap-6">
          <FieldGroup>
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
          <fieldset className="flex flex-col gap-3">
            <legend className="font-medium">选择真实衣物</legend>
            <p className="text-muted-foreground">
              按选中顺序保存，最多 20 件。
            </p>
            {itemsLoading ? (
              <p
                role="status"
                className="flex items-center gap-2 text-muted-foreground"
              >
                <Spinner /> 正在读取衣橱…
              </p>
            ) : items.length === 0 ? (
              <p className="text-muted-foreground">衣橱为空，请先添加衣物。</p>
            ) : (
              <div className="divide-y divide-border rounded-2xl border border-border px-4">
                {items.map((item) => {
                  const selected = form.items.some(
                    (selection) => selection.item_id === item.id,
                  )
                  const unavailable = item.availability !== 'wearable'
                  return (
                    <div key={item.id} className="py-2">
                      <label className="flex min-h-11 cursor-pointer items-center gap-3">
                        <input
                          type="checkbox"
                          className="size-5 shrink-0"
                          checked={selected}
                          disabled={
                            pending || (!selected && form.items.length >= 20)
                          }
                          onChange={(event) =>
                            toggleItem(item, event.target.checked)
                          }
                        />
                        <span className="min-w-0 break-words">
                          {item.name} · {availabilityLabels[item.availability]}
                        </span>
                      </label>
                      {selected && unavailable && (
                        <label className="ml-8 flex min-h-11 cursor-pointer items-center gap-3 text-muted-foreground">
                          <input
                            type="checkbox"
                            className="size-5 shrink-0"
                            checked={form.confirmedUnavailableIDs.includes(
                              item.id,
                            )}
                            disabled={pending}
                            onChange={(event) =>
                              onChange({
                                ...form,
                                confirmedUnavailableIDs: event.target.checked
                                  ? [...form.confirmedUnavailableIDs, item.id]
                                  : form.confirmedUnavailableIDs.filter(
                                      (id) => id !== item.id,
                                    ),
                              })
                            }
                          />
                          <span>我知道这件衣物当前不可穿，仍加入计划</span>
                        </label>
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
          </fieldset>
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
