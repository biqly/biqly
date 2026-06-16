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

const skip = new Set(['src/lib/cardClasses.ts'])

function libImportPath(file, module) {
  const depth = file.split('/').length - 2
  return `${'../'.repeat(depth)}lib/${module}`
}

function ensureImports(content, file) {
  const needsLegacy = content.includes('legacyCardClass(')
  const needsCn = content.includes('cn(')
  if (!needsLegacy && !needsCn) {
    return content
  }
  const hasLegacy = /import\s+\{[^}]*legacyCardClass/.test(content)
  const hasCn = /import\s+\{[^}]*\bcn\b[^}]*\}\s+from/.test(content)
  const lines = []
  if (needsLegacy && !hasLegacy) {
    lines.push(`import { legacyCardClass } from '${libImportPath(file, 'cardClasses')}'`)
  }
  if (needsCn && !hasCn) {
    lines.push(`import { cn } from '${libImportPath(file, 'cn')}'`)
  }
  if (lines.length === 0) {
    return content
  }
  const insert = `${lines.join('\n')}\n`
  const m = content.match(/^import .+$/m)
  if (m) {
    const idx = content.indexOf(m[0])
    const lineEnd = content.indexOf('\n', idx)
    return content.slice(0, lineEnd + 1) + insert + content.slice(lineEnd + 1)
  }
  return insert + content
}

let changed = 0
for (const file of files) {
  if (skip.has(file)) {
    continue
  }
  let content = fs.readFileSync(file, 'utf8')
  if (!/\bcard(-[a-z-]+)?\b/.test(content)) {
    continue
  }
  const orig = content

  content = content.replace(
    /className=\{\['([^']*\bcard[^']*)',\s*className\]\.filter\(Boolean\)\.join\(' '\)\}/g,
    (_, cardPart) => {
      const escaped = cardPart.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
      return `className={cn(legacyCardClass('${escaped}'), className)}`
    },
  )

  content = content.replace(
    /className=\{`([^`$]*\bcard[^`$]*)\$\{([^}]+)\}([^`]*)`\}/g,
    (_, cardPart, expr, tail) => {
      const card = `${cardPart.trim()}${tail ? ` ${tail.trim()}` : ''}`.trim()
      const escaped = card.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
      const extra = expr.trim()
      if (extra) {
        return `className={cn(legacyCardClass('${escaped}'), ${extra})}`
      }
      return `className={legacyCardClass('${escaped}')}`
    },
  )

  content = content.replace(/className=\{`([^`]*\bcard[^`]*)`\}/g, (_, card) => {
    const escaped = card.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
    return `className={legacyCardClass('${escaped}')}`
  })

  content = content.replace(/className="([^"]*\bcard[^"]*)"/g, (_, cls) => {
    if (cls.includes('modal-card')) {
      return `className="${cls}"`
    }
    if (cls.includes('modeling-table-card')) {
      return `className="${cls}"`
    }
    const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
    return `className={legacyCardClass('${escaped}')}`
  })

  if (content !== orig) {
    content = ensureImports(content, file)
    fs.writeFileSync(file, content)
    changed++
    console.log('updated', file)
  }
}
console.log('done', changed, 'files')
