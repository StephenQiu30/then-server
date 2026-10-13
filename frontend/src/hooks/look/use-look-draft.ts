'use client'

import { useSyncExternalStore } from 'react'
import type { LookDraft } from '@/lib/look/model'

const key = 'wearlog.try-on.sample-draft.v1'
// Read the previous brand key until the next successful save migrates the draft.
const legacyKey = 'chuanjian.try-on.sample-draft.v1'
const eventName = 'wearlog-look-draft'
function subscribe(callback: () => void) {
  window.addEventListener('storage', callback)
  window.addEventListener(eventName, callback)
  return () => {
    window.removeEventListener('storage', callback)
    window.removeEventListener(eventName, callback)
  }
}
function snapshot() {
  try {
    return localStorage.getItem(key) ?? localStorage.getItem(legacyKey)
  } catch {
    return null
  }
}
function parseDraft(raw: string | null): LookDraft | null {
  if (!raw) return null
  try {
    const value: unknown = JSON.parse(raw)
    if (
      typeof value === 'object' &&
      value !== null &&
      'name' in value &&
      typeof value.name === 'string' &&
      'scene' in value &&
      typeof value.scene === 'string'
    ) {
      return { name: value.name.slice(0, 40), scene: value.scene.slice(0, 60) }
    }
  } catch {
    /* Invalid local draft is ignored; no remote user data is involved. */
  }
  return null
}

export function useLookDraft() {
  const raw = useSyncExternalStore(subscribe, snapshot, () => null)
  return {
    draft: parseDraft(raw),
    save(name: string, scene: string) {
      localStorage.setItem(key, JSON.stringify({ name, scene }))
      try {
        localStorage.removeItem(legacyKey)
      } catch {
        /* Saved draft remains available under the new key. */
      }
      window.dispatchEvent(new Event(eventName))
    },
  }
}
