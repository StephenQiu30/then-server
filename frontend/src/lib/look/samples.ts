import type { LookDraft, LookImage, LookSummary } from '@/lib/look/model'

// Prepared design assets are never presented as results of a user's selection.
export const sampleLookComparison: { before: LookImage; after: LookImage } = {
  before: {
    src: '/look-samples/original.png',
    alt: '原穿搭：米白针织与炭灰长裤',
    label: '原穿搭',
    description: '米白针织 · 炭灰长裤',
  },
  after: {
    src: '/look-samples/result.png',
    alt: '试穿效果：海军蓝衬衫与靛蓝牛仔裤',
    label: '试穿效果',
    description: '海军蓝衬衫 · 靛蓝牛仔裤',
  },
}

export function sampleLookFromDraft(draft: LookDraft): LookSummary {
  return {
    ...draft,
    image: {
      ...sampleLookComparison.after,
      alt: `${draft.name}，已收藏的示例搭配`,
    },
    href: '/looks/sample',
    sourceLabel: '本地示例',
  }
}
