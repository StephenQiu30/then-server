export function photoValidationError(file: Pick<File, 'type' | 'size'>) {
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type))
    return '请选择 JPG、PNG 或 WebP 图片。'
  if (file.size > 10 * 1024 * 1024)
    return '图片超过 10 MB，请选择小一些的照片。'
  if (!file.size) return '这张图片是空文件，请重新选择。'
  return ''
}
