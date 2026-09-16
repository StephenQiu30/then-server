import {
  Badge,
  Box,
  Card,
  Container,
  Flex,
  Heading,
  Text,
} from '@radix-ui/themes'
import styles from './page.module.css'

export default function HomePage() {
  return (
    <main className={styles.shell}>
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
                <dl className={styles.stackList}>
                  <div>
                    <dt>应用与路由</dt>
                    <dd>Next.js App Router</dd>
                  </div>
                  <div>
                    <dt>界面</dt>
                    <dd>Radix UI Themes</dd>
                  </div>
                  <div>
                    <dt>数据请求</dt>
                    <dd>Axios + TanStack Query</dd>
                  </div>
                  <div>
                    <dt>接口契约</dt>
                    <dd>Umi OpenAPI → Axios 适配器</dd>
                  </div>
                  <div>
                    <dt>工程质量</dt>
                    <dd>TypeScript + ESLint + Prettier</dd>
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
