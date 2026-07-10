import { autocompletion } from '@codemirror/autocomplete'
import { markdown } from '@codemirror/lang-markdown'
import { languages } from '@codemirror/language-data'
import CodeMirror from '@uiw/react-codemirror'
import { useMemo } from 'react'

import { buildMdCompletionSource } from './mdCompletion'

interface MarkdownEditorProps {
  value: string
  onChange?: (value: string) => void
  readOnly?: boolean
  folder?: string
  ariaLabel: string
  minHeight?: string
}

// MarkdownEditor is the WrenAI-style source editor: line numbers, markdown
// syntax highlighting, and folder-aware completion (frontmatter keys, md
// snippets). readOnly mode doubles as the "Source" view.
export function MarkdownEditor({
  value,
  onChange,
  readOnly = false,
  folder = '',
  ariaLabel,
  minHeight = '20rem',
}: MarkdownEditorProps) {
  const extensions = useMemo(
    () => [
      markdown({ codeLanguages: languages }),
      autocompletion({ override: [buildMdCompletionSource(folder)] }),
    ],
    [folder],
  )
  return (
    <div aria-label={ariaLabel} className="cm-knowledge min-h-0">
      <CodeMirror
        value={value}
        onChange={onChange}
        readOnly={readOnly}
        editable={!readOnly}
        extensions={extensions}
        basicSetup={{
          lineNumbers: true,
          foldGutter: false,
          highlightActiveLine: !readOnly,
          highlightActiveLineGutter: !readOnly,
        }}
        style={{ minHeight }}
        theme="none"
      />
    </div>
  )
}
