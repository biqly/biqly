import type { Completion, CompletionContext, CompletionResult } from '@codemirror/autocomplete'

// Folder-specific frontmatter keys the md intellisense offers inside the
// `---` block — mirrors what the publish extraction reads per folder.
const COMMON_FM_KEYS = ['title', 'description', 'type']
const FOLDER_FM_KEYS: Record<string, string[]> = {
  glossary: ['term', 'aliases', 'maps_to_type', 'maps_to_name'],
  instructions: [],
  metrics: ['unit', 'grain'],
  'sql-pairs': ['question', 'sql'],
}

// Markdown body snippets offered outside the frontmatter block.
const BODY_SNIPPETS: Completion[] = [
  { label: '## Usage notes', type: 'keyword', info: 'Section the agent reads for routing hints' },
  { label: '```sql', type: 'keyword', apply: '```sql\n\n```', info: 'SQL code block' },
  {
    label: '---',
    type: 'keyword',
    apply: '---\ntitle: \ndescription: \n---\n',
    info: 'Frontmatter block',
  },
  {
    label: '| column | description |',
    type: 'keyword',
    apply: '| Kolon | Açıklama |\n| --- | --- |\n|  |  |',
    info: 'Table',
  },
]

// isInFrontmatter reports whether the given document offset sits inside the
// leading `---` ... `---` block.
export function isInFrontmatter(doc: string, pos: number): boolean {
  if (!doc.startsWith('---')) {
    return false
  }
  const end = doc.indexOf('\n---', 3)
  if (end === -1) {
    return pos >= 3
  }
  return pos > 3 && pos <= end
}

function frontmatterCompletions(folder: string): Completion[] {
  const keys = [...COMMON_FM_KEYS, ...(FOLDER_FM_KEYS[folder] ?? [])]
  return keys.map((key) => ({
    label: `${key}:`,
    type: 'property',
    apply: key === 'aliases' ? 'aliases: []' : `${key}: `,
  }))
}

// buildMdCompletionSource returns the CompletionSource powering the md
// intellisense: frontmatter keys inside the `---` block (per folder),
// markdown snippets in the body.
export function buildMdCompletionSource(folder: string) {
  const fmOptions = frontmatterCompletions(folder)
  return (context: CompletionContext): CompletionResult | null => {
    const word = context.matchBefore(/[\w#`>|-]*/)
    if (!word || (word.from === word.to && !context.explicit)) {
      return null
    }
    const doc = context.state.doc.toString()
    const options = isInFrontmatter(doc, context.pos) ? fmOptions : BODY_SNIPPETS
    return { from: word.from, options, validFor: /^[\w#`>|-]*$/ }
  }
}
