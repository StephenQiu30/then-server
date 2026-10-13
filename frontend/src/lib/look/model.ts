export type LookDraft = { name: string; scene: string }

export type LookImage = {
  src: string
  alt: string
  label: string
  description: string
}

export type LookSummary = LookDraft & {
  image: LookImage
  href: string
  sourceLabel: string
}
