import fs from 'fs'
import path from 'path'

const root = 'src'
const files = []

const MODELING_TOKENS =
  /\bmodeling-(?:page|toolbar|shell|palette|editor|side|kicker|join|tab|group|delete|add|rename|zoom|canvas|lines|table|column|type|schema|section|base|empty|pill|menu|status|subgroup)\b/

function walk(dir) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name)
    if (ent.isDirectory()) {
      walk(p)
    } else if (ent.name.endsWith('.tsx') || ent.name.endsWith('.ts')) {
      files.push(p)
    }
  }
}
walk(root)

const skip = new Set(['src/lib/modelingClasses.ts'])

function libImportPath(file, module) {
  const depth = file.split('/').length - 2
  return `${'../'.repeat(depth)}lib/${module}`
}

function ensureImports(content, file) {
  const needsLegacy = content.includes('legacyModelingClass(')
  const needsCn = content.includes('cn(')
  if (!needsLegacy && !needsCn) {
    return content
  }
  const hasLegacy = /import\s+\{[^}]*legacyModelingClass/.test(content)
  const hasCn = /import\s+\{[^}]*\bcn\b[^}]*\}\s+from/.test(content)
  const lines = []
  if (needsLegacy && !hasLegacy) {
    lines.push(`import { legacyModelingClass } from '${libImportPath(file, 'modelingClasses')}'`)
  }
  if (needsCn && !hasCn) {
    lines.push(`import { cn } from '${libImportPath(file, 'cn')}'`)
  }
  if (lines.length === 0) {
    return content
  }
  const insert = `${lines.join('\n')}\n`
  const importBlockRe =
    /^(?:import[\s\S]*?;\s*\n|import\s+[\s\S]*?from\s+['"][^'"]+['"]\s*;?\s*\n)+/m
  const match = content.match(importBlockRe)
  if (match) {
    const end = match.index + match[0].length
    return content.slice(0, end) + insert + content.slice(end)
  }
  return insert + content
}

function toLegacyCall(cls) {
  const escaped = cls.trim().replace(/'/g, "\\'")
  return `legacyModelingClass('${escaped}')`
}

let changed = 0
for (const file of files) {
  if (skip.has(file)) {
    continue
  }
  let content = fs.readFileSync(file, 'utf8')
  if (!MODELING_TOKENS.test(content)) {
    continue
  }
  const orig = content

  content = content.replace(/className="([^"]+)"/g, (_, cls) => {
    if (!MODELING_TOKENS.test(cls)) {
      return `className="${cls}"`
    }
    return `className={${toLegacyCall(cls)}}`
  })

  if (content !== orig) {
    content = ensureImports(content, file)
    fs.writeFileSync(file, content)
    changed++
    console.log('updated', file)
  }
}
console.log('done', changed, 'files')
