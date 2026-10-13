'use client'

import { useState, type ReactNode } from 'react'
import Image from 'next/image'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldError,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { useLookDraft } from '@/hooks/look/use-look-draft'
import { sampleLookComparison } from '@/lib/look/samples'

export function SaveLookDialog({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('蓝调日常')
  const [scene, setScene] = useState('日常 · 通勤')
  const [error, setError] = useState('')
  const { draft, save } = useLookDraft()
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        setOpen(value)
        setError('')
        if (value && draft) {
          setName(draft.name)
          setScene(draft.scene)
        }
      }}
    >
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>把这套留给下次</DialogTitle>
          <DialogDescription>
            保存示例草稿，不计入实际穿着次数。
          </DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault()
            if (!name.trim()) {
              setError('给这套搭配起个名字。')
              return
            }
            try {
              save(name.trim(), scene.trim())
              setOpen(false)
              toast.success('示例草稿已保存在此浏览器')
            } catch {
              setError('浏览器未能保存草稿，请检查存储权限后重试。')
            }
          }}
        >
          <div className="flex justify-center rounded-lg bg-surface-muted">
            <Image
              src={sampleLookComparison.after.src}
              width={128}
              height={192}
              alt="要保存的蓝调日常示例搭配"
              className="h-48 w-32 object-contain"
            />
          </div>
          <FieldGroup>
            <Field data-invalid={!!error}>
              <FieldLabel htmlFor="look-name">搭配名称</FieldLabel>
              <Input
                id="look-name"
                value={name}
                maxLength={40}
                onChange={(event) => {
                  setName(event.target.value)
                  setError('')
                }}
                aria-invalid={!!error}
                aria-describedby="save-draft-error"
              />
              <FieldError id="save-draft-error">{error}</FieldError>
            </Field>
            <Field>
              <FieldLabel htmlFor="look-scene">使用场景</FieldLabel>
              <Input
                id="look-scene"
                value={scene}
                maxLength={60}
                onChange={(event) => setScene(event.target.value)}
              />
            </Field>
          </FieldGroup>
          <p className="text-xs text-muted-foreground">
            仅保存在当前浏览器，不同步到账户，也不会写入真实衣橱。
          </p>
          <Button type="submit">保存示例草稿</Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
