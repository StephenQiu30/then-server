import { copyFile, mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const target = new URL('../public/wardrobe-processing/', import.meta.url)
await mkdir(target, { recursive: true })
for (const name of [
  'ort-wasm-simd-threaded.mjs',
  'ort-wasm-simd-threaded.wasm',
]) {
  await copyFile(
    fileURLToPath(
      new URL(`../node_modules/onnxruntime-web/dist/${name}`, import.meta.url),
    ),
    new URL(name, target),
  )
}
