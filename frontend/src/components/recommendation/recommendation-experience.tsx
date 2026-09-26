'use client'

import { useRef, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon } from 'lucide-react'
import { toast } from 'sonner'
import { PageShell } from '@/components/layout/page-shell'
import { scenes, type SceneId } from '@/components/recommendation/scenes'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useCurrentUser } from '@/hooks/account/use-current-user'
import { useOutfitPlanActions } from '@/hooks/outfit-plan/use-outfit-plans'
import { useWardrobeRecommendation } from '@/hooks/wardrobe/use-wardrobe-recommendation'
import { accountErrorMessage, accountStatus } from '@/lib/account/errors'
import { todayInTimeZone } from '@/lib/outfit-plan/form'
import { planErrorMessage } from '@/lib/outfit-plan/errors'

type Formality = NonNullable<
  API.WardrobeRecommendationRequest['formality_band']
>
type Warmth = NonNullable<API.WardrobeRecommendationRequest['warmth_band']>

const formalityOptions: { value: Formality | 'any'; label: string }[] = [
  { value: 'any', label: '不限正式度' },
  { value: 'casual', label: '休闲' },
  { value: 'smart_casual', label: '整洁日常' },
  { value: 'formal', label: '正式' },
]
const warmthOptions: { value: Warmth | 'any'; label: string }[] = [
  { value: 'any', label: '不限保暖' },
  { value: 'light', label: '轻薄' },
  { value: 'medium', label: '适中' },
  { value: 'warm', label: '偏暖' },
]
const sceneDefaults: Record<
  SceneId,
  { formality: Formality | 'any'; extras: string[] }
> = {
  daily: { formality: 'any', extras: [] },
  commute: { formality: 'smart_casual', extras: [] },
  friends: { formality: 'any', extras: [] },
  date: { formality: 'smart_casual', extras: [] },
  travel: { formality: 'any', extras: ['walking'] },
  relax: { formality: 'casual', extras: [] },
}
const reasonLabels: Record<string, string> = {
  real_owner_items: '来自你本人衣橱里的真实衣物',
  explicit_packed_items: '包含你本次明确允许的已打包衣物',
  complete_one_piece_path: '连衣装与鞋组成完整搭配',
  complete_separate_path: '上装、下装与鞋组成完整搭配',
  confirmed_formality: '符合你确认的正式度',
  confirmed_warmth: '符合你确认的保暖要求',
  confirmed_rain_shoes: '鞋履符合你确认的雨天要求',
  confirmed_walking_shoes: '鞋履符合你确认的步行要求',
}
const uncertaintyLabels: Record<string, string> = {
  some_formality_unknown: '部分衣物的正式度未知',
  some_warmth_unknown: '部分衣物的保暖程度未知',
  some_rain_suitability_unknown: '部分衣物的雨天适用性未知',
  some_walking_suitability_unknown: '部分衣物的步行适用性未知',
}
const gapLabels: Record<string, string> = {
  no_available_items: '衣橱里还没有当前可用的衣物。',
  missing_shoes: '还缺少当前可用的鞋。',
  missing_top_or_one_piece: '还缺少当前可用的上装或连衣装。',
  missing_bottom_for_top: '有上装，但还缺少当前可用的下装。',
  confirmed_constraints_conflict: '现有衣物无法同时满足这些已确认的条件。',
}
const categoryLabels: Record<API.WardrobeItemResponse['category'], string> = {
  top: '上装',
  bottom: '下装',
  one_piece: '连衣装',
  outerwear: '外套',
  shoes: '鞋',
  bag: '包',
  accessory: '配饰',
}

export function RecommendationExperience({
  initialScene,
}: {
  initialScene: SceneId
}) {
  const currentUser = useCurrentUser()
  const plans = useOutfitPlanActions()
  const recommendWardrobe = useWardrobeRecommendation()
  const scene = scenes.find((item) => item.id === initialScene) ?? scenes[0]
  const [formality, setFormality] = useState<Formality | 'any'>(
    sceneDefaults[initialScene].formality,
  )
  const [warmth, setWarmth] = useState<Warmth | 'any'>('any')
  const [extras, setExtras] = useState<string[]>(
    sceneDefaults[initialScene].extras,
  )
  const [pending, setPending] = useState(false)
  const [savePending, setSavePending] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [result, setResult] =
    useState<API.WardrobeRecommendationResponse | null>(null)
  const [submitted, setSubmitted] =
    useState<API.WardrobeRecommendationRequest | null>(null)
  const [saved, setSaved] = useState<Record<string, string>>({})
  const planIDs = useRef(new Map<string, string>())
  const requestVersion = useRef(0)

  function changeConstraints(change: () => void) {
    requestVersion.current += 1
    change()
    setPending(false)
    setResult(null)
    setSubmitted(null)
    setError(null)
    setSaveError(null)
    setSaved({})
    planIDs.current.clear()
  }

  async function recommend(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!currentUser.data || pending) return
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
    const request: API.WardrobeRecommendationRequest = {
      local_date: todayInTimeZone(timeZone),
      time_zone: timeZone,
      formality_band: formality === 'any' ? undefined : formality,
      warmth_band: warmth === 'any' ? undefined : warmth,
      requires_rain_suitability: extras.includes('rain'),
      requires_walking_suitability: extras.includes('walking'),
      include_packed_items: extras.includes('packed'),
    }
    setPending(true)
    const version = ++requestVersion.current
    setError(null)
    setSaveError(null)
    setResult(null)
    try {
      const response = await recommendWardrobe(request)
      if (requestVersion.current !== version) return
      setSubmitted(request)
      setResult(response)
    } catch (cause) {
      if (requestVersion.current !== version) return
      setError(
        accountStatus(cause) === 401
          ? '会话已失效，请重新登录后再试。'
          : accountErrorMessage(cause),
      )
    } finally {
      if (requestVersion.current === version) setPending(false)
    }
  }

  async function savePlan(
    candidate: API.WardrobeRecommendationCandidateResponse,
    key: string,
  ) {
    if (!submitted || savePending || saved[key]) return
    const items = candidate.items as API.WardrobeItemResponse[]
    const id = planIDs.current.get(key) ?? crypto.randomUUID()
    planIDs.current.set(key, id)
    setSavePending(key)
    setSaveError(null)
    try {
      const plan = await plans.create({
        id,
        local_date: submitted.local_date,
        time_zone: submitted.time_zone,
        context_summary: scene.label,
        items: items.map((item) => ({
          item_id: item.id,
          revision: item.revision,
        })),
        confirmed_unavailable_ids: items
          .filter((item) => item.availability === 'packed')
          .map((item) => item.id),
      })
      setSaved((previous) => ({ ...previous, [key]: plan.id }))
      planIDs.current.delete(key)
      toast('已保存为穿搭计划')
    } catch (cause) {
      setSaveError(planErrorMessage(cause))
    } finally {
      setSavePending(null)
    }
  }

  const candidates = (result?.candidates ??
    []) as API.WardrobeRecommendationCandidateResponse[]
  const resultGridClass =
    candidates.length === 1
      ? 'grid max-w-2xl gap-4'
      : candidates.length === 2
        ? 'grid gap-4 min-[900px]:grid-cols-2'
        : 'grid gap-4 min-[900px]:grid-cols-3'
  return (
    <PageShell>
      <header className="flex flex-col gap-5">
        <Button asChild variant="link" className="self-start">
          <Link href="/">
            <ArrowLeftIcon data-icon="inline-start" />
            重新选择场景
          </Link>
        </Button>
        <div className="flex flex-col gap-3">
          <Badge variant="outline">真实衣橱推荐</Badge>
          <h1 className="page-title">{scene.label}，穿什么？</h1>
          <p className="max-w-2xl text-muted-foreground">
            先确认条件，再从你当前可用的衣物中选择。未知属性不会被猜测，结果也不会自动保存或记作实际穿着。
          </p>
        </div>
      </header>

      {currentUser.isPending ? (
        <p
          role="status"
          className="flex items-center gap-2 text-muted-foreground"
        >
          <Spinner />
          正在检查会话…
        </p>
      ) : currentUser.isError && accountStatus(currentUser.error) === 401 ? (
        <Alert>
          <AlertTitle>登录后使用真实推荐</AlertTitle>
          <AlertDescription className="flex flex-col items-start gap-3">
            推荐只读取你自己的衣橱。
            <Button asChild>
              <Link href="/login">
                登录账户 <ArrowRightIcon data-icon="inline-end" />
              </Link>
            </Button>
          </AlertDescription>
        </Alert>
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
        <>
          <Card>
            <CardHeader>
              <CardTitle>
                <h2>确认本次搭配条件</h2>
              </CardTitle>
              <CardDescription>
                场景可预填条件；你可以在推荐前修改。只有明确选择的条件会参与筛选。
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(event) => void recommend(event)}
                className="flex flex-col gap-6"
              >
                <div className="flex flex-col gap-3">
                  <p id="formality-label" className="font-medium">
                    正式度
                  </p>
                  <ToggleGroup
                    type="single"
                    disabled={savePending !== null}
                    aria-labelledby="formality-label"
                    value={formality}
                    onValueChange={(value) => {
                      if (value)
                        changeConstraints(() =>
                          setFormality(value as Formality | 'any'),
                        )
                    }}
                    className="flex w-full flex-wrap justify-start gap-2"
                  >
                    {formalityOptions.map((option) => (
                      <ToggleGroupItem
                        key={option.value}
                        value={option.value}
                        variant="outline"
                      >
                        {option.label}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </div>
                <div className="flex flex-col gap-3">
                  <p id="warmth-label" className="font-medium">
                    保暖程度
                  </p>
                  <ToggleGroup
                    type="single"
                    disabled={savePending !== null}
                    aria-labelledby="warmth-label"
                    value={warmth}
                    onValueChange={(value) => {
                      if (value)
                        changeConstraints(() =>
                          setWarmth(value as Warmth | 'any'),
                        )
                    }}
                    className="flex w-full flex-wrap justify-start gap-2"
                  >
                    {warmthOptions.map((option) => (
                      <ToggleGroupItem
                        key={option.value}
                        value={option.value}
                        variant="outline"
                      >
                        {option.label}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </div>
                <div className="flex flex-col gap-3">
                  <p id="extras-label" className="font-medium">
                    额外条件
                  </p>
                  <ToggleGroup
                    type="multiple"
                    disabled={savePending !== null}
                    aria-labelledby="extras-label"
                    value={extras}
                    onValueChange={(value) =>
                      changeConstraints(() => setExtras(value))
                    }
                    className="flex w-full flex-wrap justify-start gap-2"
                  >
                    <ToggleGroupItem value="rain" variant="outline">
                      需要适合雨天的鞋
                    </ToggleGroupItem>
                    <ToggleGroupItem value="walking" variant="outline">
                      需要适合步行的鞋
                    </ToggleGroupItem>
                    <ToggleGroupItem value="packed" variant="outline">
                      本次允许已打包衣物
                    </ToggleGroupItem>
                  </ToggleGroup>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    type="submit"
                    disabled={pending || savePending !== null}
                  >
                    {pending ? (
                      <Spinner data-icon="inline-start" />
                    ) : (
                      <ArrowRightIcon data-icon="inline-end" />
                    )}
                    {pending ? '正在检查衣橱…' : '生成真实衣橱推荐'}
                  </Button>
                  <span className="text-sm text-muted-foreground">
                    只读取衣橱；生成本身不会创建计划。
                  </span>
                </div>
              </form>
            </CardContent>
          </Card>

          {error && (
            <Alert variant="destructive" role="alert">
              <AlertTitle>推荐未完成</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          {saveError && (
            <Alert variant="destructive" role="alert">
              <AlertTitle>计划未保存</AlertTitle>
              <AlertDescription>
                {saveError}请核对衣橱后重新推荐。
              </AlertDescription>
            </Alert>
          )}
          {result && (
            <section
              aria-labelledby="result-title"
              className="flex flex-col gap-5"
            >
              <div className="flex flex-col gap-2">
                <h2
                  id="result-title"
                  className="text-2xl font-semibold tracking-tight"
                >
                  {candidates.length
                    ? '从你的衣橱找到这些搭配'
                    : '这次暂时没有完整搭配'}
                </h2>
                <p className="text-sm text-muted-foreground">
                  {candidates.length
                    ? `共 ${candidates.length} 套，按当前衣橱和确认条件生成。`
                    : '条件不会被悄悄放宽，也不会补造不存在的衣物。'}
                </p>
              </div>
              {candidates.length === 0 ? (
                <Alert>
                  <AlertTitle>找不到符合条件的组合</AlertTitle>
                  <AlertDescription className="flex flex-col items-start gap-3">
                    {gapLabels[result.gap ?? ''] ??
                      '请检查衣橱状态和本次条件。'}
                    <Button asChild variant="outline">
                      <Link href="/wardrobe">查看我的衣橱</Link>
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : (
                <div className={resultGridClass}>
                  {candidates.map((candidate, index) => {
                    const items = candidate.items as API.WardrobeItemResponse[]
                    const key = items.map((item) => item.id).join(':')
                    return (
                      <Card key={key} className="h-full">
                        <CardHeader>
                          <CardTitle>
                            <h3>搭配 {index + 1}</h3>
                          </CardTitle>
                          <CardDescription>
                            本人的真实衣物 · 请求时状态
                          </CardDescription>
                        </CardHeader>
                        <CardContent className="flex flex-1 flex-col gap-5">
                          <ul className="divide-y divide-border">
                            {items.map((item) => (
                              <li
                                key={item.id}
                                className="flex items-center justify-between gap-3 py-3 first:pt-0"
                              >
                                <span className="min-w-0 font-medium break-words">
                                  {item.name}
                                </span>
                                <span className="flex shrink-0 flex-wrap gap-1">
                                  <Badge variant="secondary">
                                    {categoryLabels[item.category]}
                                  </Badge>
                                  {item.availability === 'packed' && (
                                    <Badge variant="outline">已打包</Badge>
                                  )}
                                </span>
                              </li>
                            ))}
                          </ul>
                          <div className="flex flex-col gap-2">
                            <p className="text-sm font-medium">推荐依据</p>
                            <ul className="flex flex-col gap-1 text-sm text-muted-foreground">
                              {(candidate.reasons as string[]).map((reason) => (
                                <li key={reason}>
                                  · {reasonLabels[reason] ?? reason}
                                </li>
                              ))}
                            </ul>
                          </div>
                          {(candidate.uncertainties as string[]).length > 0 && (
                            <div className="flex flex-col gap-2">
                              <p className="text-sm font-medium">尚不确定</p>
                              <ul className="flex flex-col gap-1 text-sm text-muted-foreground">
                                {(candidate.uncertainties as string[]).map(
                                  (value) => (
                                    <li key={value}>
                                      · {uncertaintyLabels[value] ?? value}
                                    </li>
                                  ),
                                )}
                              </ul>
                            </div>
                          )}
                        </CardContent>
                        <CardFooter className="justify-between gap-3 bg-card">
                          {saved[key] ? (
                            <span
                              role="status"
                              className="flex items-center gap-2 text-sm"
                            >
                              <CheckIcon className="size-4" />
                              已保存为计划
                            </span>
                          ) : (
                            <Button
                              type="button"
                              variant="outline"
                              disabled={savePending !== null}
                              onClick={() => void savePlan(candidate, key)}
                            >
                              {savePending === key && (
                                <Spinner data-icon="inline-start" />
                              )}
                              保存为计划
                            </Button>
                          )}
                          {saved[key] && (
                            <Link
                              href="/plans"
                              className="text-sm text-link underline-offset-4 hover:underline"
                            >
                              查看计划
                            </Link>
                          )}
                        </CardFooter>
                      </Card>
                    )
                  })}
                </div>
              )}
            </section>
          )}
        </>
      )}
    </PageShell>
  )
}
