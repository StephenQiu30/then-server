import type { ReactNode } from 'react'

export function PageShell({ children }: { children: ReactNode }) {
  return (
    <main
      id="main-content"
      tabIndex={-1}
      className="mx-auto flex min-h-svh w-full max-w-4xl flex-col justify-center gap-12 px-gutter py-12 min-[641px]:px-8 min-[641px]:py-section"
    >
      {children}
    </main>
  )
}
