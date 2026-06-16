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

const skip = new Set(['src/lib/formClasses.ts'])

const modelingFiles = new Set([
  'src/components/modeling/JoinEditor.tsx',
  'src/components/modeling/EditDimensionModal.tsx',
  'src/components/modeling/BaseSwapModal.tsx',
  'src/components/modeling/AddMetricModal.tsx',
  'src/components/modeling/ModelingToolbar.tsx',
  'src/components/modeling/ModelingModals.tsx',
  'src/components/modeling/AddMetricSimpleFields.tsx',
])

function libImportPath(file, module) {
  const depth = file.split('/').length - 2
  return `${'../'.repeat(depth)}lib/${module}`
}

function ensureImports(content, file) {
  const needsLegacy = content.includes('legacyFormClass(')
  const needsModeling = content.includes('modelingFormGroupClass')
  const needsCn = content.includes('cn(')
  if (!needsLegacy && !needsCn && !needsModeling) {
    return content
  }
  const hasLegacy = /import\s+\{[^}]*legacyFormClass/.test(content)
  const hasModeling = /import\s+\{[^}]*modelingFormGroupClass/.test(content)
  const hasCn = /import\s+\{[^}]*\bcn\b[^}]*\}\s+from/.test(content)
  const lines = []
  if (needsLegacy && !hasLegacy) {
    lines.push(`import { legacyFormClass } from '${libImportPath(file, 'formClasses')}'`)
  }
  if (needsModeling && !hasModeling) {
    lines.push(`import { modelingFormGroupClass } from '${libImportPath(file, 'formClasses')}'`)
  }
  if (needsCn && !hasCn) {
    lines.push(`import { cn } from '${libImportPath(file, 'cn')}'`)
  }
  if (lines.length === 0) {
    return content
  }
  const insert = `${lines.join('\n')}\n`
  let lastImportEnd = 0
  const importRe = /^import .+$/gm
  let m
  while ((m = importRe.exec(content)) !== null) {
    lastImportEnd = content.indexOf('\n', m.index) + 1
  }
  if (lastImportEnd > 0) {
    return content.slice(0, lastImportEnd) + insert + content.slice(lastImportEnd)
  }
  return insert + content
}

function formGroupReplacement(file, cls) {
  if (modelingFiles.has(file) && cls.trim() === 'form-group') {
    return 'className={modelingFormGroupClass}'
  }
  const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
  return `className={legacyFormClass('${escaped}')}`
}

let changed = 0
for (const file of files) {
  if (skip.has(file)) {
    continue
  }
  let content = fs.readFileSync(file, 'utf8')
  if (!/\b(form-group|form-field|form-label|className="input")\b/.test(content)) {
    continue
  }
  const orig = content

  content = content.replace(/className="form-group"/g, () =>
    formGroupReplacement(file, 'form-group'),
  )

  content = content.replace(/className="([^"]*\bform-group\b[^"]*)"/g, (_, cls) => {
    if (cls.trim() === 'form-group') {
      return formGroupReplacement(file, cls)
    }
    const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
    return `className={legacyFormClass('${escaped}')}`
  })

  content = content.replace(/className="form-field"/g, `className={legacyFormClass('form-field')}`)

  content = content.replace(/className="([^"]*\bform-field\b[^"]*)"/g, (_, cls) => {
    if (cls.trim() === 'form-field') {
      return `className={legacyFormClass('form-field')}`
    }
    const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
    return `className={legacyFormClass('${escaped}')}`
  })

  content = content.replace(/className="form-label"/g, `className={legacyFormClass('form-label')}`)

  content = content.replace(/className="([^"]*\bform-label\b[^"]*)"/g, (_, cls) => {
    const escaped = cls.trim().replace(/\\/g, '\\\\').replace(/'/g, "\\'")
    return `className={legacyFormClass('${escaped}')}`
  })

  content = content.replace(/className="input"/g, `className={legacyFormClass('input')}`)

  if (content !== orig) {
    content = ensureImports(content, file)
    fs.writeFileSync(file, content)
    changed++
    console.log('updated', file)
  }
}
console.log('done', changed, 'files')
