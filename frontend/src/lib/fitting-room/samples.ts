import type { PreservedGarment, GarmentOption } from '@/lib/wardrobe/selection'

export const fittingRoomGarments = [
  {
    id: 'shirt',
    name: '海军蓝棉质衬衫',
    category: 'top',
    categoryLabel: '上装',
    color: '海军蓝',
    image: '/wardrobe-samples/navy-shirt.png',
  },
  {
    id: 'knit',
    name: '米白针织衫',
    category: 'top',
    categoryLabel: '上装',
    color: '米白色',
    image: '/wardrobe-samples/cream-knit.png',
  },
  {
    id: 'jeans',
    name: '经典直筒牛仔裤',
    category: 'bottom',
    categoryLabel: '下装',
    color: '靛蓝色',
    image: '/wardrobe-samples/blue-jeans.png',
  },
] as const satisfies readonly GarmentOption[]

export const originalGarments = {
  top: {
    id: 'original-knit',
    name: '米白针织衫',
    image: '/wardrobe-samples/cream-knit.png',
    categoryLabel: '上装',
  },
  bottom: {
    id: 'original-trousers',
    name: '炭灰垂感长裤',
    image: '/wardrobe-samples/charcoal-trousers.png',
    categoryLabel: '下装',
  },
  shoes: {
    id: 'original-shoes',
    name: '白色低帮运动鞋',
    image: '/wardrobe-samples/low-top-sneakers.png',
    categoryLabel: '鞋履',
  },
} as const satisfies Record<string, PreservedGarment>
