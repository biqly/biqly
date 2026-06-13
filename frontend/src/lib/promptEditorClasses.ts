import { cn } from './cn'

export const promptEditorContainerClass = 'relative w-full'

export const promptEditorUnderlayClass = cn(
  'prompt-editor-underlay absolute top-0 left-0 w-full h-full m-0',
  'py-2 px-3 border border-transparent font-mono text-[0.85rem] leading-[1.4]',
  'whitespace-pre-wrap break-words overflow-hidden text-foreground pointer-events-none bg-transparent text-left',
)

export const promptEditorTextareaClass = cn(
  'prompt-editor-textarea relative z-[2] w-full !bg-transparent !text-transparent',
  'caret-foreground font-mono text-[0.85rem] leading-[1.4] py-2 px-3',
  'border border-border rounded-lg shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)] resize-y',
  'transition-[border-color,box-shadow] duration-120 ease',
  'focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none',
)
