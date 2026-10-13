'use client'

import { useRef } from 'react'
import {
  completeMediaUpload,
  createMediaUpload,
  getMedia,
  getWardrobeImage,
  getWardrobeOriginal,
} from '@/api/privateMedia'
import request from '@/lib/api/request'
import { WardrobePhotoError } from '@/lib/wardrobe/photo'

// Only the existing signed upload is sent to MinIO; no session cookie or storage credential.
export function useWardrobePhotoUpload() {
  const attempts = useRef(
    new WeakMap<File, { id?: string; version?: string }>(),
  )
  return {
    async upload(file: File): Promise<string> {
      let current = attempts.current.get(file)
      if (!current) {
        current = {}
        attempts.current.set(file, current)
      }
      if (!current.id) {
        const bitmap = await createImageBitmap(file, {
          imageOrientation: 'from-image',
        }).catch(() => {
          throw new WardrobePhotoError(
            '照片无法解码，请重新选择 JPG、PNG 或 WebP 图片。',
          )
        })
        try {
          if (bitmap.width * bitmap.height > 24_000_000)
            throw new WardrobePhotoError(
              '照片超过 2400 万像素，请缩小后再上传。',
            )
          const jpeg = file // Preserve exact source bytes, including PNG transparency.
          const digest = Array.from(
            new Uint8Array(
              await crypto.subtle.digest('SHA-256', await jpeg.arrayBuffer()),
            ),
            (byte) => byte.toString(16).padStart(2, '0'),
          ).join('')
          const upload = await createMediaUpload({
            purpose: 'wardrobe_image',
            content_type:
              file.type as API.CreateMediaUploadRequest['content_type'],
            byte_size: jpeg.size,
            sha256: digest,
          })
          const headers = { ...upload.headers }
          delete headers['Content-Length'] // The browser supplies the signed length from the Blob.
          let version = ''
          await request(upload.url, {
            method: 'PUT',
            data: jpeg,
            headers,
            withCredentials: false,
            timeout: 30_000,
            onResponseHeaders: (response) => {
              version = String(response['x-amz-version-id'] ?? '')
            },
          })
          if (!version)
            throw new WardrobePhotoError('存储未返回照片版本，请重试。')
          current.id = upload.media.id
          current.version = version
        } finally {
          bitmap.close()
        }
      }
      await completeMediaUpload(
        { media_id: current.id! },
        { version_id: current.version! },
      )
      const deadline = Date.now() + 30_000
      do {
        const media = await getMedia({ media_id: current.id! })
        if (media.status === 'ready') return media.id
        if (['rejected', 'deleting', 'deleted'].includes(media.status))
          throw new WardrobePhotoError('照片未通过图片校验，请选择其他照片。')
        await new Promise((resolve) => setTimeout(resolve, 600))
      } while (Date.now() < deadline)
      throw new WardrobePhotoError(
        '照片仍在处理，请稍后再次保存。已上传的照片会继续使用。',
      )
    },
  }
}

export function useWardrobePhotoSource() {
  return {
    async read(
      id: string,
      original: boolean,
    ): Promise<{ file: File; legacy: boolean }> {
      let legacy = false
      const options = {
        responseType: 'blob' as const,
        onResponseHeaders: (headers: Record<string, unknown>) => {
          legacy = headers['x-original-source'] === 'normalized-legacy'
        },
      }
      const body = original
        ? await getWardrobeOriginal({ media_id: id }, options)
        : await getWardrobeImage({ media_id: id }, options)
      const blob = body as unknown as Blob
      return {
        file: new File([blob], 'wardrobe-source', { type: blob.type }),
        legacy,
      }
    },
  }
}
