import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'
import ts from 'typescript'

const root = fileURLToPath(new URL('../../', import.meta.url))
const sourceRoot = path.join(root, 'src')
const config = ts.readConfigFile(
  path.join(root, 'tsconfig.json'),
  ts.sys.readFile,
)
const { options } = ts.parseJsonConfigFileContent(config.config, ts.sys, root)

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const name = path.join(directory, entry.name)
    return entry.isDirectory() ? sourceFiles(name) : [name]
  })
}

function layer(file: string): string {
  const relative = path.relative(sourceRoot, file).split(path.sep).join('/')
  if (relative.startsWith('components/ui/')) return 'ui'
  return relative.split('/')[0]
}

test('component modules expose a matching responsibility name', () => {
  const violations: string[] = []
  const files = [
    ...sourceFiles(path.join(sourceRoot, 'components')),
    ...sourceFiles(path.join(sourceRoot, 'layout')),
  ]
  for (const file of files) {
    // Registry modules retain shadcn's canonical compound exports.
    if (layer(file) === 'ui' || !file.endsWith('.tsx')) continue
    const name = path.basename(file, '.tsx')
    const expected = name
      .split('-')
      .map((word) => word[0].toUpperCase() + word.slice(1))
      .join('')
    const ast = ts.createSourceFile(
      file,
      readFileSync(file, 'utf8'),
      ts.ScriptTarget.Latest,
      true,
    )
    const hasPrimaryExport = ast.statements.some(
      (node) =>
        ts.isFunctionDeclaration(node) &&
        node.name?.text === expected &&
        node.modifiers?.some(
          (modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword,
        ),
    )
    // Navigation deliberately composes desktop and mobile counterparts.
    if (!hasPrimaryExport && name !== 'site-navigation') {
      violations.push(
        `${path.relative(sourceRoot, file)} must export ${expected}`,
      )
    }
    if (/(?:-panel|-shell|-experience)$/.test(name)) {
      violations.push(
        `${name}: use the actual View, Layout, Form or display responsibility`,
      )
    }
  }
  assert.deepEqual(violations, [])
})

test('reusable look and person presentation accepts data instead of owning fixtures or storage', () => {
  for (const name of [
    'look/look-card',
    'look/look-comparison',
    'person/person-preview',
  ]) {
    const file = path.join(sourceRoot, 'components', `${name}.tsx`)
    const source = readFileSync(file, 'utf8')
    const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true)
    for (const node of ast.statements) {
      if (
        !ts.isImportDeclaration(node) ||
        !ts.isStringLiteral(node.moduleSpecifier)
      )
        continue
      assert.doesNotMatch(
        node.moduleSpecifier.text,
        /(?:\/samples$|@\/hooks\/|@\/api\/)/,
        name,
      )
    }
    assert.doesNotMatch(
      source,
      /localStorage|\/look-samples\/(?:original|result)\.png/,
      name,
    )
  }
})

test('source directories and imports follow PROJECT boundaries', () => {
  const allowed: Record<string, string[]> = {
    app: [
      'app',
      'components',
      'layout',
      'ui',
      'providers',
      'hooks',
      'lib',
      'api',
    ],
    components: ['components', 'layout', 'ui', 'hooks', 'lib'],
    layout: ['layout', 'ui', 'lib'],
    ui: ['ui'],
    providers: ['providers', 'lib'],
    hooks: ['hooks', 'lib', 'api'],
    lib: ['lib'],
    api: ['api', 'lib'],
  }
  const violations: string[] = []
  for (const file of sourceFiles(sourceRoot)) {
    const relative = path.relative(sourceRoot, file)
    const from = layer(file)
    if (!(from in allowed))
      violations.push(`Unexpected src directory: ${relative}`)
    if (/\.(test|spec)\.[cm]?[jt]sx?$/.test(file)) {
      violations.push(`Tests belong in tests/: ${relative}`)
    }
    // Generated code is owned by Umi, including its namespace index and names.
    if (from === 'api' || !/\.[cm]?[jt]sx?$/.test(file)) continue
    const ast = ts.createSourceFile(
      file,
      readFileSync(file, 'utf8'),
      ts.ScriptTarget.Latest,
      true,
    )
    function check(specifier: ts.Node | undefined) {
      if (!specifier || !ts.isStringLiteralLike(specifier)) return
      const value = specifier.text
      if (
        !value.startsWith('.') &&
        !value.startsWith('@/') &&
        !path.isAbsolute(value)
      )
        return
      const resolved = ts.resolveModuleName(
        value,
        file,
        options,
        ts.sys,
      ).resolvedModule
      // TypeScript handles missing modules; side-effect CSS imports also land here.
      const target =
        resolved?.resolvedFileName ??
        (value.startsWith('@/')
          ? path.resolve(sourceRoot, value.slice(2))
          : path.resolve(path.dirname(file), value))
      const local = path.relative(sourceRoot, target).split(path.sep).join('/')
      const to = layer(target)
      const uiUtility = from === 'ui' && local === 'lib/utils.ts'
      if (!(allowed[from]?.includes(to) || uiUtility)) {
        violations.push(`${relative}: ${value} crosses ${from} -> ${to}`)
      }
      if (value === '@/api' || local === 'api/index.ts') {
        violations.push(`${relative}: import a specific generated API module`)
      }
    }
    function visit(node: ts.Node) {
      if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) {
        check(node.moduleSpecifier)
      } else if (
        ts.isCallExpression(node) &&
        (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
          (ts.isIdentifier(node.expression) &&
            node.expression.text === 'require'))
      ) {
        check(node.arguments[0])
      } else if (
        ts.isImportTypeNode(node) &&
        ts.isLiteralTypeNode(node.argument)
      ) {
        check(node.argument.literal)
      } else if (ts.isExternalModuleReference(node)) {
        check(node.expression)
      }
      ts.forEachChild(node, visit)
    }
    visit(ast)
  }
  assert.deepEqual(violations, [], violations.join('\n'))
})
