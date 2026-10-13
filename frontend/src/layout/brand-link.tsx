import Image from 'next/image'
import Link from 'next/link'

export function BrandLink() {
  return (
    <Link
      href="/"
      aria-label="衣序，返回首页"
      className="inline-flex min-h-11 items-center gap-2 whitespace-nowrap"
    >
      <Image
        src="/brand/wearlog-symbol.svg"
        alt=""
        width={24}
        height={24}
        priority
      />
      <span className="text-[19px] font-semibold tracking-tight">WEARLOG</span>
      <span className="hidden text-xs text-muted-foreground min-[360px]:inline">
        衣序
      </span>
    </Link>
  )
}
