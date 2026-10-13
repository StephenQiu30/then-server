import assert from 'node:assert/strict'
import { test } from 'node:test'
import { replaceGarment } from '../../src/lib/outfit-plan/selection.ts'
import { sampleImagePath } from '../../src/lib/wardrobe/samples.ts'

const wardrobe = [
  { id: 'top-a', category: 'top', revision: 1 },
  { id: 'top-b', category: 'top', revision: 3 },
  { id: 'bottom', category: 'bottom', revision: 2 },
  { id: 'shoes', category: 'shoes', revision: 4 },
  { id: 'dress', category: 'one_piece', revision: 1 },
] as API.WardrobeItemResponse[]
const selection = wardrobe
  .slice(0, 1)
  .concat(wardrobe.slice(2, 4))
  .map((item) => ({ item_id: item.id, revision: item.revision }))

test('swapping one top preserves trousers and shoes exactly and never duplicates a selection', () => {
  const next = replaceGarment(selection, wardrobe[1], wardrobe)
  assert.deepEqual(next, [
    selection[1],
    selection[2],
    { item_id: 'top-b', revision: 3 },
  ])
  assert.deepEqual(replaceGarment(next, wardrobe[1], wardrobe), next)
  assert.equal(selection[0].item_id, 'top-a')
})

test('a dress replaces separates; returning to a top preserves shoes', () => {
  const dress = replaceGarment(selection, wardrobe[4], wardrobe)
  assert.deepEqual(
    dress.map((item) => item.item_id),
    ['shoes', 'dress'],
  )
  assert.deepEqual(
    replaceGarment(dress, wardrobe[0], wardrobe).map((item) => item.item_id),
    ['shoes', 'top-a'],
  )
})

test('sample image references only resolve the five bundled cutouts', () => {
  assert.equal(
    sampleImagePath('cream-knit'),
    '/wardrobe-samples/cream-knit.png',
  )
  assert.equal(sampleImagePath('https://example.test/private'), null)
  assert.equal(sampleImagePath('../private'), null)
  assert.equal(sampleImagePath(), null)
})
