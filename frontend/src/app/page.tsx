import {
  Badge,
  Box,
  Card,
  Container,
  Flex,
  Heading,
  Text,
} from '@radix-ui/themes'

export default function HomePage() {
  return (
    <main className="grid min-h-screen place-items-center py-16">
      <Container size="2" px="5">
        <Flex direction="column" gap="6">
          <Flex direction="column" gap="3">
            <Badge color="teal" size="2" variant="soft">
              Next.js App Router
            </Badge>
            <Heading as="h1" size="8">
              于是 OOTD
            </Heading>
            <Text color="gray" size="4">
              Next.js 前端服务已就绪，业务页面将在对应账户管理切片中接入。
            </Text>
          </Flex>

          <Card size="3">
            <Flex direction="column" gap="4">
              <Heading as="h2" size="4">
                基础设施
              </Heading>
              <Box asChild>
                <dl className="m-0 grid gap-4">
                  <div className="grid grid-cols-1 gap-1 min-[32rem]:grid-cols-[minmax(7rem,0.6fr)_minmax(0,1.4fr)]">
                    <dt className="font-semibold text-[var(--gray-11)]">
                      应用与路由
                    </dt>
                    <dd className="m-0">Next.js App Router</dd>
                  </div>
                  <div className="grid grid-cols-1 gap-1 min-[32rem]:grid-cols-[minmax(7rem,0.6fr)_minmax(0,1.4fr)]">
                    <dt className="font-semibold text-[var(--gray-11)]">
                      界面
                    </dt>
                    <dd className="m-0">Radix UI Themes + Tailwind CSS</dd>
                  </div>
                  <div className="grid grid-cols-1 gap-1 min-[32rem]:grid-cols-[minmax(7rem,0.6fr)_minmax(0,1.4fr)]">
                    <dt className="font-semibold text-[var(--gray-11)]">
                      数据请求
                    </dt>
                    <dd className="m-0">Axios + TanStack Query</dd>
                  </div>
                  <div className="grid grid-cols-1 gap-1 min-[32rem]:grid-cols-[minmax(7rem,0.6fr)_minmax(0,1.4fr)]">
                    <dt className="font-semibold text-[var(--gray-11)]">
                      接口契约
                    </dt>
                    <dd className="m-0">Umi OpenAPI → Axios 适配器</dd>
                  </div>
                  <div className="grid grid-cols-1 gap-1 min-[32rem]:grid-cols-[minmax(7rem,0.6fr)_minmax(0,1.4fr)]">
                    <dt className="font-semibold text-[var(--gray-11)]">
                      工程质量
                    </dt>
                    <dd className="m-0">TypeScript + ESLint + Prettier</dd>
                  </div>
                </dl>
              </Box>
            </Flex>
          </Card>
        </Flex>
      </Container>
    </main>
  )
}
