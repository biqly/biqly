import fs from 'fs'
import path from 'path'

const root = 'src'
const files = []

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

const skip = new Set(['src/lib/layoutClasses.ts', 'src/App.tsx'])

function libImportPath(file, module) {
  const depth = file.split('/').length - 2
  return `${'../'.repeat(depth)}lib/${module}`
}

function ensureImports(content, file) {
  const needsLegacy = content.includes('legacyLayoutClass(')
  const needsCn = content.includes('cn(')
  if (!needsLegacy && !needsCn) {
    return content
  }
  const hasLegacy = /import\s+\{[^}]*legacyLayoutClass/.test(content)
  const hasCn = /import\s+\{[^}]*\bcn\b[^}]*\}\s+from/.test(content)
  const lines = []
  if (needsLegacy && !hasLegacy) {
    lines.push(`import { legacyLayoutClass } from '${libImportPath(file, 'layoutClasses')}'`)
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

let changed = 0
for (const file of files) {
  if (skip.has(file)) {
    continue
  }
  let content = fs.readFileSync(file, 'utf8')
  if (!/\bpage-stack\b/.test(content)) {
    continue
  }
  const orig = content

  content = content.replace(
    /className="page-stack"/g,
    `className={legacyLayoutClass('page-stack')}`,
  )

  content = content.replace(/className="([^"]*\bpage-stack\b[^"]*)"/g, (_, cls) => {
    const escaped = cls.trim().replace(/'/g, "\\'")
    return `className={legacyLayoutClass('${escaped}')}`
  })

  content = content.replace(
    /className=\{`([^`$]*\bpage-stack\b[^`$]*)\$\{([^}]+)\}([^`]*)`\}/g,
    (_, before, expr, after) => {
      const layout = `${before.trim()}${after ? ` ${after.trim()}` : ''}`.trim()
      const escaped = layout.replace(/'/g, "\\'")
      const extra = expr.trim()
      if (extra) {
        return `className={cn(legacyLayoutClass('${escaped}'), ${extra})}`
      }
      return `className={legacyLayoutClass('${escaped}')}`
    },
  )

  content = content.replace(/className=\{`([^`]*\bpage-stack\b[^`]*)`\}/g, (_, cls) => {
    const escaped = cls.trim().replace(/'/g, "\\'")
    return `className={legacyLayoutClass('${escaped}')}`
  })

  if (content !== orig) {
    content = ensureImports(content, file)
    fs.writeFileSync(file, content)
    changed++
    console.log('updated', file)
  }
}
console.log('done', changed, 'files')
