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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import {
  availabilityLabels,
  categoryLabels,
  type WardrobeForm,
} from '@/lib/wardrobe/form'

const attributeFields: {
  key: keyof API.WardrobeAttributesRequest
  label: string
  options: { value: string; label: string }[]
}[] = [
  {
    key: 'formality_band',
    label: '正式度',
    options: [
      { value: 'casual', label: '休闲' },
      { value: 'smart_casual', label: '适度正式' },
      { value: 'formal', label: '正式' },
    ],
  },
  {
    key: 'warmth_band',
    label: '保暖感受',
    options: [
      { value: 'light', label: '轻薄' },
      { value: 'medium', label: '适中' },
      { value: 'warm', label: '保暖' },
    ],
  },
  {
    key: 'rain_use',
    label: '雨天适用',
    options: [
      { value: 'suitable', label: '适用' },
      { value: 'unsuitable', label: '不适用' },
    ],
  },
  {
    key: 'walking_use',
    label: '步行适用',
    options: [
      { value: 'suitable', label: '适用' },
      { value: 'unsuitable', label: '不适用' },
    ],
  },
]

export function WardrobeEditor({
  form,
  onChange,
  onSubmit,
  editing,
  pending,
  nameError,
  onCancel,
  onReload,
}: {
  form: WardrobeForm
  onChange: (form: WardrobeForm) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  editing: boolean
  pending: boolean
  nameError: string | null
  onCancel: () => void
  onReload: () => void
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{editing ? '编辑衣物' : '添加衣物'}</CardTitle>
        <CardDescription>
          只记录你确认拥有的衣物；照片和扩展属性可留空。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} noValidate className="flex flex-col gap-6">
          <FieldGroup>
            <Field data-invalid={!!nameError}>
              <FieldLabel htmlFor="wardrobe-name">名称</FieldLabel>
              <Input
                id="wardrobe-name"
                name="name"
                required
                value={form.name}
                onChange={(event) =>
                  onChange({ ...form, name: event.target.value })
                }
                aria-invalid={!!nameError}
                aria-describedby={nameError ? 'wardrobe-name-error' : undefined}
              />
              {nameError && (
                <FieldError id="wardrobe-name-error">{nameError}</FieldError>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="wardrobe-category">类别</FieldLabel>
              <NativeSelect
                id="wardrobe-category"
                name="category"
                value={form.category}
                onChange={(event) =>
                  onChange({
                    ...form,
                    category: event.target.value as WardrobeForm['category'],
                  })
                }
              >
                {Object.entries(categoryLabels).map(([value, label]) => (
                  <NativeSelectOption key={value} value={value}>
                    {label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="wardrobe-availability">当前状态</FieldLabel>
              <NativeSelect
                id="wardrobe-availability"
                name="availability"
                value={form.availability}
                onChange={(event) =>
                  onChange({
                    ...form,
                    availability: event.target
                      .value as WardrobeForm['availability'],
                  })
                }
              >
                {Object.entries(availabilityLabels).map(([value, label]) => (
                  <NativeSelectOption key={value} value={value}>
                    {label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
              <FieldDescription>
                待洗、借出和已收纳不会被当成当前可穿衣物。
              </FieldDescription>
            </Field>
          </FieldGroup>
          <details className="rounded-2xl border border-border p-4">
            <summary className="cursor-pointer font-medium">
              可选确认属性
            </summary>
            <p className="mt-2 text-muted-foreground">
              不确定时保持“未知”，不会自动推断。
            </p>
            <FieldGroup className="mt-5">
              {attributeFields.map((field) => (
                <Field key={field.key}>
                  <FieldLabel htmlFor={`wardrobe-${field.key}`}>
                    {field.label}
                  </FieldLabel>
                  <NativeSelect
                    id={`wardrobe-${field.key}`}
                    value={form.attributes[field.key] ?? ''}
                    onChange={(event) =>
                      onChange({
                        ...form,
                        attributes: {
                          ...form.attributes,
                          [field.key]: event.target.value || undefined,
                        },
                      })
                    }
                  >
                    <NativeSelectOption value="">未知</NativeSelectOption>
                    {field.options.map((option) => (
                      <NativeSelectOption
                        key={option.value}
                        value={option.value}
                      >
                        {option.label}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </Field>
              ))}
            </FieldGroup>
          </details>
          <div className="flex flex-wrap gap-3">
            <Button type="submit" disabled={pending}>
              {pending && <Spinner data-icon="inline-start" />}
              {pending ? '正在保存…' : editing ? '保存修改' : '添加到衣橱'}
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
                  重新加载衣物
                </Button>
              </>
            )}
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
