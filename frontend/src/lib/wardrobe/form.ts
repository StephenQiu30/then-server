export type WardrobeForm = {
  name: string
  category: API.CreateWardrobeItemRequest['category']
  availability: API.CreateWardrobeItemRequest['availability']
  attributes: API.WardrobeAttributesRequest
}

export const categoryLabels: Record<WardrobeForm['category'], string> = {
  top: '上衣',
  bottom: '下装',
  one_piece: '连衣装',
  outerwear: '外套',
  shoes: '鞋履',
  bag: '包',
  accessory: '配饰',
}

export const availabilityLabels: Record<WardrobeForm['availability'], string> =
  {
    wearable: '可穿',
    laundry: '待洗',
    lent_out: '借出',
    packed: '已收纳',
  }

export function emptyWardrobeForm(): WardrobeForm {
  return { name: '', category: 'top', availability: 'wearable', attributes: {} }
}

export function wardrobeFormFromItem(
  item: Pick<
    API.WardrobeItemResponse,
    'name' | 'category' | 'availability' | 'attributes'
  >,
): WardrobeForm {
  return {
    name: item.name,
    category: item.category,
    availability: item.availability,
    attributes: {
      formality_band: item.attributes?.formality_band?.value,
      warmth_band: item.attributes?.warmth_band?.value,
      rain_use: item.attributes?.rain_use?.value,
      walking_use: item.attributes?.walking_use?.value,
    },
  }
}

export function wardrobeNameError(name: string): string | null {
  const value = name.trim()
  if (
    !value ||
    Array.from(value).length > 80 ||
    /[\u0000-\u001f\u007f-\u009f]/u.test(value)
  ) {
    return '请输入不超过 80 个字符且不含控制字符的名称。'
  }
  return null
}
