// Existing Codex cutouts. These are fictional samples, not photos of owned items.
export const garmentSamples = [
  {
    key: 'cream-knit',
    name: '米白针织衫',
    category: 'top',
    id: '6a236c7c-1e1e-4e48-8750-21d94a330001',
  },
  {
    key: 'navy-shirt',
    name: '海军蓝衬衫',
    category: 'top',
    id: '6a236c7c-1e1e-4e48-8750-21d94a330002',
  },
  {
    key: 'charcoal-trousers',
    name: '炭灰长裤',
    category: 'bottom',
    id: '6a236c7c-1e1e-4e48-8750-21d94a330003',
  },
  {
    key: 'blue-jeans',
    name: '蓝色牛仔裤',
    category: 'bottom',
    id: '6a236c7c-1e1e-4e48-8750-21d94a330004',
  },
  {
    key: 'white-sneakers',
    name: '白色运动鞋',
    category: 'shoes',
    id: '6a236c7c-1e1e-4e48-8750-21d94a330005',
  },
] as const

export function sampleImagePath(key?: string): string | null {
  return garmentSamples.some((sample) => sample.key === key)
    ? `/wardrobe-samples/${key}.png`
    : null
}
