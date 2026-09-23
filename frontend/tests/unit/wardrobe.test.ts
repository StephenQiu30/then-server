import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  emptyWardrobeForm,
  wardrobeFormFromItem,
  wardrobeNameError,
} from '../../src/lib/wardrobe/form.ts'

test('wardrobe form keeps unknown attributes unknown', () => {
  const form = emptyWardrobeForm()
  assert.deepEqual(form.attributes, {})
  const fromItem = wardrobeFormFromItem({
    name: '白衬衫',
    category: 'top',
    availability: 'wearable',
    attributes: {
      formality_band: { value: 'formal', source: 'user_confirmed' },
      warmth_band: null,
      rain_use: null,
      walking_use: null,
    } as unknown as API.WardrobeAttributesResponse,
  })
  assert.equal(fromItem.attributes.formality_band, 'formal')
  assert.equal(fromItem.attributes.warmth_band, undefined)
})

test('wardrobe name counts Unicode characters and rejects control characters', () => {
  assert.equal(wardrobeNameError('🧥'.repeat(80)), null)
  assert.equal(wardrobeNameError('🧥'.repeat(81)) !== null, true)
  assert.equal(wardrobeNameError(' 衬衫\n'), null)
  assert.equal(wardrobeNameError('衬\n衫') !== null, true)
})
