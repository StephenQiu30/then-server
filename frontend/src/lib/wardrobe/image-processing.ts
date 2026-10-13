import { WardrobePhotoError } from './photo'
import type { InferenceSession } from 'onnxruntime-web/wasm'

export type CropInsets = {
  top: number
  right: number
  bottom: number
  left: number
}
export const emptyCrop: CropInsets = { top: 0, right: 0, bottom: 0, left: 0 }
let session: Promise<InferenceSession> | undefined

function canvas(width: number, height: number) {
  const result = document.createElement('canvas')
  result.width = width
  result.height = height
  const context = result.getContext('2d', { willReadFrequently: true })
  if (!context) throw new WardrobePhotoError('浏览器无法处理图片。')
  return { result, context }
}

async function bitmap(file: File) {
  const image = await createImageBitmap(file, {
    imageOrientation: 'from-image',
  }).catch(() => {
    throw new WardrobePhotoError('照片无法读取，请重新选择。')
  })
  if (image.width * image.height > 24_000_000) {
    image.close()
    throw new WardrobePhotoError('照片超过 2400 万像素，请缩小后重试。')
  }
  return image
}

function pngFile(result: HTMLCanvasElement): Promise<File> {
  return new Promise((resolve, reject) =>
    result.toBlob((blob) => {
      if (!blob || blob.size > 12 * 1024 * 1024) {
        reject(new WardrobePhotoError('处理图过大，请裁小后重试。'))
        return
      }
      resolve(new File([blob], 'garment.png', { type: 'image/png' }))
    }, 'image/png'),
  )
}

export function cropBounds(width: number, height: number, crop: CropInsets) {
  if (
    Object.values(crop).some(
      (value) => !Number.isFinite(value) || value < 0 || value > 45,
    ) ||
    crop.left + crop.right >= 100 ||
    crop.top + crop.bottom >= 100
  )
    throw new WardrobePhotoError('裁剪范围无效。')
  return {
    x: Math.floor((width * crop.left) / 100),
    y: Math.floor((height * crop.top) / 100),
    width: Math.max(
      1,
      Math.round((width * (100 - crop.left - crop.right)) / 100),
    ),
    height: Math.max(
      1,
      Math.round((height * (100 - crop.top - crop.bottom)) / 100),
    ),
  }
}

export async function cropPhoto(file: File, crop: CropInsets): Promise<File> {
  const image = await bitmap(file)
  try {
    const box = cropBounds(image.width, image.height, crop)
    const ratio = Math.min(1, 2048 / Math.max(box.width, box.height))
    const { result, context } = canvas(
      Math.max(1, Math.round(box.width * ratio)),
      Math.max(1, Math.round(box.height * ratio)),
    )
    context.drawImage(
      image,
      box.x,
      box.y,
      box.width,
      box.height,
      0,
      0,
      result.width,
      result.height,
    )
    return await pngFile(result)
  } finally {
    image.close()
  }
}

export async function removeBackground(file: File): Promise<File> {
  const ort = await import('onnxruntime-web/wasm')
  ort.env.wasm.wasmPaths = new URL(
    '/wardrobe-processing/',
    window.location.href,
  ).href
  ort.env.wasm.numThreads = 1
  if (!session)
    session = ort.InferenceSession.create('/wardrobe-processing/u2netp.onnx', {
      executionProviders: ['wasm'],
    }).catch((cause) => {
      session = undefined
      throw cause
    })
  const model = await session
  const image = await bitmap(file)
  let input: InstanceType<typeof ort.Tensor> | undefined
  let outputs: Awaited<ReturnType<InferenceSession['run']>> | undefined
  try {
    const small = canvas(320, 320)
    small.context.drawImage(image, 0, 0, 320, 320)
    const pixels = small.context.getImageData(0, 0, 320, 320).data
    const size = 320 * 320,
      data = new Float32Array(3 * size),
      mean = [0.485, 0.456, 0.406],
      std = [0.229, 0.224, 0.225]
    let maximum = 1
    for (let i = 0; i < size; i++)
      for (let c = 0; c < 3; c++) maximum = Math.max(maximum, pixels[4 * i + c])
    for (let i = 0; i < size; i++)
      for (let c = 0; c < 3; c++)
        data[c * size + i] = (pixels[4 * i + c] / maximum - mean[c]) / std[c]
    input = new ort.Tensor('float32', data, [1, 3, 320, 320])
    outputs = await model.run({ [model.inputNames[0]]: input })
    const prediction = outputs[model.outputNames[0]].data as Float32Array
    let min = Infinity,
      max = -Infinity
    for (let i = 0; i < size; i++) {
      min = Math.min(min, prediction[i])
      max = Math.max(max, prediction[i])
    }
    if (max - min < 0.001)
      throw new WardrobePhotoError(
        '未找到清楚的衣物轮廓，请换用简单背景或先裁剪。',
      )
    const mask = small.context.createImageData(320, 320)
    for (let i = 0; i < size; i++) {
      mask.data[4 * i] = 255
      mask.data[4 * i + 1] = 255
      mask.data[4 * i + 2] = 255
      mask.data[4 * i + 3] = Math.round(
        255 *
          Math.max(
            0,
            Math.min(1, ((prediction[i] - min) / (max - min) - 0.08) / 0.84),
          ),
      )
    }
    small.context.putImageData(mask, 0, 0)
    const ratio = Math.min(1, 2048 / Math.max(image.width, image.height))
    const { result, context } = canvas(
      Math.round(image.width * ratio),
      Math.round(image.height * ratio),
    )
    context.drawImage(image, 0, 0, result.width, result.height)
    context.globalCompositeOperation = 'destination-in'
    context.drawImage(small.result, 0, 0, result.width, result.height)
    context.globalCompositeOperation = 'source-over'
    // Trim transparent margins after segmentation so garments use comparable flat-lay space.
    const rgba = context.getImageData(0, 0, result.width, result.height).data
    let left = result.width,
      top = result.height,
      right = -1,
      bottom = -1
    for (let y = 0; y < result.height; y++)
      for (let x = 0; x < result.width; x++)
        if (rgba[(y * result.width + x) * 4 + 3] > 24) {
          left = Math.min(left, x)
          top = Math.min(top, y)
          right = Math.max(right, x)
          bottom = Math.max(bottom, y)
        }
    if (right < left)
      throw new WardrobePhotoError('未找到衣物，请恢复原图或先裁剪。')
    const padding = Math.ceil(Math.max(right - left, bottom - top) * 0.03)
    left = Math.max(0, left - padding)
    top = Math.max(0, top - padding)
    right = Math.min(result.width - 1, right + padding)
    bottom = Math.min(result.height - 1, bottom + padding)
    const trimmed = canvas(right - left + 1, bottom - top + 1)
    trimmed.context.drawImage(
      result,
      left,
      top,
      trimmed.result.width,
      trimmed.result.height,
      0,
      0,
      trimmed.result.width,
      trimmed.result.height,
    )
    return await pngFile(trimmed.result)
  } finally {
    input?.dispose()
    if (outputs) for (const tensor of Object.values(outputs)) tensor.dispose()
    image.close()
  }
}
