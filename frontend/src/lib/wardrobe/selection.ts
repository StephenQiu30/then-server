export type GarmentOption = {
  id: string
  name: string
  category: 'top' | 'bottom'
  categoryLabel: string
  color: string
  image: string
}

export type PreservedGarment = {
  id: string
  name: string
  image: string
  categoryLabel: string
}
