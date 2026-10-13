export const siteLinks = [
  { href: '/', label: '试衣间' },
  { href: '/wardrobe', label: '衣橱' },
  { href: '/looks', label: '搭配' },
  { href: '/wear', label: '记录' },
] as const

export const headerLinks = [
  ...siteLinks,
  { href: '/account', label: '我的' },
] as const

export function isSiteLinkCurrent(href: string, pathname: string) {
  if (href === '/') return pathname === '/'
  if (href === '/account') {
    return ['/account', '/login', '/register'].includes(pathname)
  }
  return pathname === href
}
