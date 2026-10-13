import type { PlanSelection } from './form.ts'

// Replacing a category keeps every other category, ID and revision unchanged.
// A one-piece and separates are mutually exclusive in this flat-lay editor.
export function replaceGarment(
  selections: PlanSelection[],
  item: API.WardrobeItemResponse,
  wardrobe: API.WardrobeItemResponse[],
): PlanSelection[] {
  const categories = new Map(
    wardrobe.map((entry) => [entry.id, entry.category]),
  )
  const conflicts = new Set<API.WardrobeItemResponse['category']>([
    item.category,
  ])
  if (item.category === 'one_piece') {
    conflicts.add('top')
    conflicts.add('bottom')
  }
  if (item.category === 'top' || item.category === 'bottom')
    conflicts.add('one_piece')
  return [
    ...selections.filter((entry) => {
      const category = categories.get(entry.item_id)
      return !category || !conflicts.has(category)
    }),
    { item_id: item.id, revision: item.revision },
  ]
}
