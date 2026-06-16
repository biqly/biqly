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

const skip = new Set([
  'src/lib/feedbackClasses.ts',
  'src/components/ui/EmptyState.tsx',
  'src/components/ui/LoadingOverlay.tsx',
  'src/components/ui/ErrorAlert.tsx',
  'src/components/ui/ChartContainer.tsx',
  'src/components/aiQuery/assistantMessageCardSections.tsx',
])

const legacyTokens = [
  'error',
  'error--top-gap',
  'success',
  'loading-text',
  'loading-overlay-wrap',
  'loading-overlay',
  'loading-overlay-spinner',
  'warning-panel',
  'chart-container',
  'sql-preview',
  'empty-state',
  'ui-empty-state',
  'ui-empty-state--inline',
]

const tokenRe = new RegExp(
  `\\b(?:${legacyTokens.map((t) => t.replace(/\\/g, '\\\\').replace(/-/g, '\\-')).join('|')})\\b`,
)

function libImportPath(file) {
  const depth = file.split('/').length - 2
  return `${'../'.repeat(depth)}lib/feedbackClasses`
}

function ensureImports(content, file) {
  if (!content.includes('legacyFeedbackClass(')) {
    return content
  }
  if (/import\s+\{[^}]*legacyFeedbackClass/.test(content)) {
    return content
  }
  const insert = `import { legacyFeedbackClass } from '${libImportPath(file)}'\n`
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
  if (!tokenRe.test(content)) {
    continue
  }
  const orig = content

  for (const token of legacyTokens) {
    const escaped = token.replace(/\\/g, '\\\\').replace(/-/g, '\\-')
    content = content.replace(
      new RegExp(`className="${escaped}"`, 'g'),
      `className={legacyFeedbackClass('${token}')}`,
    )
    content = content.replace(
      new RegExp(`className='${escaped}'`, 'g'),
      `className={legacyFeedbackClass('${token}')}`,
    )
  }

  content = content.replace(
    /className="([^"]*\b(?:error|success|sql-preview|chart-container)\b[^"]*)"/g,
    (_, cls) => {
      const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
      return `className={legacyFeedbackClass('${escaped}')}`
    },
  )

  content = content.replace(
    /className=\{`([^`$]*\b(?:error|success|sql-preview|chart-container|loading-overlay)\b[^`$]*)\$\{([^}]+)\}([^`]*)`\}/g,
    (_, before, expr, after) => {
      const layout = `${before.trim()}${after ? ` ${after.trim()}` : ''}`.trim()
      const escaped = layout.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
      return `className={\`\${legacyFeedbackClass('${escaped}')} \${${expr}}${after ? ` ${after.trim()}` : ''}\`}`
    },
  )

  content = content.replace(
    /className=\{`([^`]*\b(?:error|sql-preview|chart-container)\b[^`]*)`\}/g,
    (_, cls) => {
      const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
      return `className={legacyFeedbackClass('${escaped}')}`
    },
  )

  content = content.replace(
    /className=\{`sql-preview \$\{([^}]+)\}`\}/g,
    (_, expr) => `className={\`\${legacyFeedbackClass('sql-preview')} \${${expr}}\`}`,
  )

  if (content !== orig) {
    content = ensureImports(content, file)
    fs.writeFileSync(file, content)
    changed++
    console.log('updated', file)
  }
}
console.log('done', changed, 'files')
