/* eslint-disable react-refresh/only-export-components */
import { memo, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'

import { request } from '../../hooks/useApi'
import type { LooseTFunction } from '../../i18n'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import type { ColumnRow, SemanticExprNode, SemanticModelDetail } from '../../types/semantic'

type SemanticUnaryExpr = Extract<SemanticExprNode, { type: 'unary' }>
type SemanticFunctionCallExpr = Extract<SemanticExprNode, { type: 'function_call' }>
type SemanticCaseExpr = Extract<SemanticExprNode, { type: 'case' }>
import { errorMessage } from '../../utils/error'
import { isRecord } from '../../utils/record'
// Whitelisted SQL functions with arity hints (-1 = variadic)
export interface FunctionInfo {
  name: string
  arity: number
  desc: string
}

export const ALLOWED_FUNCTIONS: FunctionInfo[] = [
  { name: 'COALESCE', arity: -1, desc: 'Returns first non-null value' },
  { name: 'CONCAT', arity: -1, desc: 'Concatenates multiple strings' },
  { name: 'UPPER', arity: 1, desc: 'Converts string to uppercase' },
  { name: 'LOWER', arity: 1, desc: 'Converts string to lowercase' },
  { name: 'ROUND', arity: 2, desc: 'Rounds number to decimal places' },
  { name: 'LENGTH', arity: 1, desc: 'Returns string length' },
  { name: 'TRIM', arity: 1, desc: 'Removes leading/trailing spaces' },
  { name: 'ABS', arity: 1, desc: 'Returns absolute value' },
  { name: 'CEIL', arity: 1, desc: 'Rounds up to nearest integer' },
  { name: 'FLOOR', arity: 1, desc: 'Rounds down to nearest integer' },
  { name: 'NULLIF', arity: 2, desc: 'Returns null if values are equal' },
  { name: 'IFNULL', arity: 2, desc: 'Returns second value if first is null' },
  { name: 'ISNULL', arity: 1, desc: 'Returns true if value is null' },
  { name: 'SUBSTRING', arity: 3, desc: 'Extracts substring' },
  { name: 'REPLACE', arity: 3, desc: 'Replaces substring occurrences' },
  { name: 'LEFT', arity: 2, desc: 'Extracts characters from left' },
  { name: 'RIGHT', arity: 2, desc: 'Extracts characters from right' },
  { name: 'DATE_TRUNC', arity: 2, desc: 'Truncates date to grain' },
  { name: 'CAST', arity: 2, desc: 'Casts a value to a type' },
  { name: 'EXTRACT', arity: 2, desc: 'Extracts a date/time part' },
]

export const AGGREGATE_FUNCTIONS: FunctionInfo[] = [
  { name: 'SUM', arity: 1, desc: 'Sum of values' },
  { name: 'AVG', arity: 1, desc: 'Average of values' },
  { name: 'MIN', arity: 1, desc: 'Minimum value' },
  { name: 'MAX', arity: 1, desc: 'Maximum value' },
  { name: 'COUNT', arity: -1, desc: 'Count rows or non-null values' },
  { name: 'COUNT_DISTINCT', arity: 1, desc: 'Count distinct values' },
]

export const BINARY_OPS = [
  { value: 'add', label: '+' },
  { value: 'subtract', label: '-' },
  { value: 'multiply', label: '*' },
  { value: 'divide', label: '/' },
  { value: 'modulo', label: '%' },
  { value: 'concat', label: 'CONCAT (||)' },
  { value: 'eq', label: '=' },
  { value: 'neq', label: '!=' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '<=' },
  { value: 'gt', label: '>' },
  { value: 'gte', label: '>=' },
  { value: 'and', label: 'AND' },
  { value: 'or', label: 'OR' },
]

export const UNARY_OPS = [
  { value: 'not', label: 'NOT' },
  { value: 'negate', label: '-' },
]

const exprAstSelectClass =
  'text-[0.8rem] py-[0.35rem] px-[0.6rem] bg-canvas border border-border-strong rounded text-foreground outline-none cursor-pointer min-w-24 transition-[border-color] duration-[120ms] ease-in-out focus:border-accent'

const exprAstOperatorSelectClass = `${exprAstSelectClass} font-bold text-warning border-[rgba(251,191,36,0.3)]`

const exprAstTypeSelectClass =
  'text-[0.72rem] font-bold uppercase text-accent bg-transparent border-0 cursor-pointer py-[0.15rem] px-2 rounded transition-[background] duration-100 ease-in-out hover:bg-white/5'

const exprAstNodeCardClass =
  'relative bg-card-raised border border-border-strong border-l-4 border-l-accent rounded-md py-3 px-4 flex flex-col gap-2 transition-[border-color,box-shadow] duration-150 ease-in-out hover:shadow-[0_4px_12px_rgba(0,0,0,0.15)] hover:border-accent/40'

const exprAstFlexRowClass =
  'flex gap-2 items-center flex-wrap [&_input]:font-mono [&_input]:text-[0.82rem]'

const exprAstAddBtnClass =
  'self-start text-[0.72rem] font-semibold py-1 px-2 border border-dashed border-accent bg-transparent text-accent rounded cursor-pointer transition-all duration-100 ease-in-out hover:bg-accent/10 hover:text-white'

const exprAstRemoveBtnClass =
  'text-[1.15rem] bg-transparent border-0 text-foreground-muted cursor-pointer py-[0.15rem] px-[0.4rem] leading-none transition-colors duration-100 ease-in-out hover:text-error'

const exprAstCaseLabelClass = 'text-[0.72rem] font-extrabold text-warning uppercase min-w-14'

const exprModeToggleBase =
  'text-[0.8rem] font-semibold py-[0.4rem] px-[0.8rem] rounded border cursor-pointer transition-all duration-150 ease-in-out'

function exprModeToggleClass(active: boolean) {
  return active
    ? `${exprModeToggleBase} border-accent bg-accent text-white shadow-[var(--accent-shadow-glow)]`
    : `${exprModeToggleBase} border-border-strong bg-canvas text-foreground-muted hover:bg-border hover:text-foreground`
}

interface ExpressionBuilderProps {
  model: SemanticModelDetail
  columns: ColumnRow[]
  initialNode?: SemanticExprNode
  initialText?: string
  allowAggregates?: boolean
  onChange: (node: SemanticExprNode, textExpression: string) => void
  t: LooseTFunction
}

export function ExpressionBuilder({
  model,
  columns,
  initialNode,
  initialText = '',
  allowAggregates = false,
  onChange,
  t,
}: ExpressionBuilderProps) {
  const [mode, setMode] = useState<'visual' | 'text'>('text')
  const [textInput, setTextInput] = useState(initialText)
  const [astNode, setAstNode] = useState<SemanticExprNode>(
    initialNode ?? { type: 'literal', value: '' },
  )

  const [compiledSQL, setCompiledSQL] = useState('')
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  // Map active tables in the model for select dropdowns
  const activeTables = useMemo(() => {
    const keys = new Set<string>()
    keys.add(`${model.base_schema}.${model.base_table}`)
    ;(model.joins ?? []).forEach((j) => {
      if (j.is_active !== false) {
        keys.add(`${j.from_schema ?? model.base_schema}.${j.from_table}`)
        keys.add(`${j.to_schema ?? model.base_schema}.${j.to_table}`)
      }
    })
    return Array.from(keys).map((k) => {
      const parts = k.split('.')
      return { schema: parts[0] ?? '', table: parts[1] ?? '' }
    })
  }, [model])

  // Filter columns based on active tables
  const modelColumns = useMemo(() => {
    return columns.filter((col) =>
      activeTables.some((t) => t.schema === col.schema_name && t.table === col.table_name),
    )
  }, [columns, activeTables])

  // Active dimensions
  const modelDimensions = useMemo(() => {
    return (model.dimensions ?? []).filter((d) => d.is_active !== false)
  }, [model])

  // Active metrics
  const modelMetrics = useMemo(() => {
    return (model.metrics ?? []).filter((m) => m.is_active !== false)
  }, [model])

  // Call backend compile endpoint
  const compileExpression = useCallback(
    async (payload: { expression?: string; expr?: SemanticExprNode }) => {
      setLoading(true)
      try {
        const { data, error } = await request<unknown>(
          'POST',
          `/api/semantic/models/${model.id}/compile-expression`,
          { ...payload, allow_aggregates: allowAggregates },
        )
        if (data !== null && isRecord(data)) {
          const sql = typeof data.sql === 'string' ? data.sql : ''
          const expr = data.expr as SemanticExprNode | undefined
          setCompiledSQL(sql)
          setErrorMsg(null)
          if (expr) {
            setAstNode(expr)
          }
          onChange(expr ?? astNode, payload.expression ?? sql)
        } else {
          setErrorMsg(error ?? 'Failed to compile expression')
          setCompiledSQL('')
        }
      } catch (err: unknown) {
        const message = errorMessage(err)
        setErrorMsg('Network error: ' + message)
        setCompiledSQL('')
      } finally {
        setLoading(false)
      }
    },
    [model.id, onChange, astNode, allowAggregates],
  )

  // Trigger compile when Visual AST changes
  const handleAstChange = useCallback(
    (newNode: SemanticExprNode) => {
      setAstNode(newNode)
      void compileExpression({ expr: newNode })
    },
    [compileExpression],
  )

  // Debounced compile for text input
  useEffect(() => {
    if (mode === 'visual') {
      return
    }
    const timer = setTimeout(() => {
      if (textInput.trim()) {
        void compileExpression({ expression: textInput })
      } else {
        setCompiledSQL('')
        setErrorMsg(null)
      }
    }, 400)
    return () => clearTimeout(timer)
  }, [textInput, mode, compileExpression])

  // Sync mode toggle
  const toggleMode = () => {
    if (mode === 'text') {
      // Sync from Text to Visual: text is already compiled to AST state, just change tab
      setMode('visual')
    } else {
      // Sync from Visual to Text: use compiled SQL as text representation
      setTextInput(compiledSQL || textInput)
      setMode('text')
    }
  }
  return (
    <div
      className={
        'bg-card border-border shadow-card-sm mt-3 flex flex-col gap-4 rounded-lg border p-5'
      }
    >
      <div className={`border-border flex gap-2 border-b pb-3`}>
        <button type="button" className={exprModeToggleClass(mode === 'text')} onClick={toggleMode}>
          {t('modeling.expr_mode_text')}
        </button>
        <button
          type="button"
          className={exprModeToggleClass(mode === 'visual')}
          onClick={toggleMode}
        >
          {t('modeling.expr_mode_visual')}
        </button>
      </div>

      <div className="max-h-96 min-h-48 overflow-y-auto p-1">
        {mode === 'visual' ? (
          <div className="flex flex-col gap-3">
            <ExpressionNodeBuilder
              node={astNode}
              onChangeNode={handleAstChange}
              activeTables={activeTables}
              modelColumns={modelColumns}
              modelDimensions={modelDimensions}
              modelMetrics={modelMetrics}
              t={t}
            />
          </div>
        ) : (
          <div className="relative w-full">
            <textarea
              id="raw-text-expression"
              className={`border-border bg-canvas text-foreground caret-foreground relative z-2 w-full resize-y rounded-md border p-2 font-mono text-[0.85rem] leading-[1.4] shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)] transition-[border-color,box-shadow] duration-120 ease-in-out focus-visible:border-(--control-focus-border) focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none`}
              value={textInput}
              onChange={(e) => setTextInput(e.target.value)}
              placeholder="e.g. sum([orders.total_amount]) - sum([orders.discount])"
              rows={4}
              disabled={loading}
            />
            <div className="text-foreground-muted mt-[0.35rem] text-[0.7rem]">
              {t('modeling.metric_intellisense_hint')}
            </div>
          </div>
        )}
      </div>

      {errorMsg && (
        <div
          className={legacyFeedbackClass(
            'bg-error/10 border-error/30 text-error mt-2 rounded-md border px-[0.8rem] py-[0.6rem] font-mono text-[0.78rem]',
          )}
        >
          {errorMsg}
        </div>
      )}

      <div className="mt-2 flex flex-col gap-2">
        <h4 className="text-foreground-muted m-0 text-[0.75rem] font-bold tracking-wide uppercase">
          {t('modeling.generated_sql')}
        </h4>
        {loading ? (
          <div className="text-foreground-muted p-3 text-[0.8rem]">{t('modeling.running')}</div>
        ) : (
          <code
            className={`bg-canvas-subtle text-success border-border block overflow-x-auto rounded-md border px-4 py-3 font-mono text-[0.85rem] wrap-break-word whitespace-pre-wrap`}
          >
            {compiledSQL || '-- Type an expression or build one visually to see SQL --'}
          </code>
        )}
      </div>
    </div>
  )
}

function getDefaultNode(
  newType: string,
  activeTables: { schema: string; table: string }[],
  modelColumns: ColumnRow[],
  modelDimensions: SemanticModelDetail['dimensions'],
  modelMetrics: SemanticModelDetail['metrics'],
): SemanticExprNode {
  switch (newType) {
    case 'literal':
      return { type: 'literal', value: '' }
    case 'column_ref':
      return {
        type: 'column_ref',
        table: activeTables[0]?.table ?? '',
        column: modelColumns[0]?.column_name ?? '',
      }
    case 'metric_ref':
      return {
        type: 'metric_ref',
        name: modelMetrics?.[0]?.name ?? '',
      }
    case 'dimension_ref':
      return {
        type: 'dimension_ref',
        name: modelDimensions?.[0]?.name ?? '',
      }
    case 'binary':
      return {
        type: 'binary',
        op: 'add',
        left: { type: 'literal', value: 0 },
        right: { type: 'literal', value: 0 },
      }
    case 'unary':
      return {
        type: 'unary',
        op: 'not',
        expr: { type: 'literal', value: true },
      }
    case 'function_call':
      return {
        type: 'function_call',
        name: 'UPPER',
        args: [{ type: 'literal', value: '' }],
      }
    case 'case':
      return {
        type: 'case',
        conditions: [
          {
            when: { type: 'literal', value: true },
            then: { type: 'literal', value: '' },
          },
        ],
        else: { type: 'literal', value: '' },
      }
    default:
      return { type: 'literal', value: '' }
  }
}

interface ExpressionNodeBuilderProps {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  activeTables: { schema: string; table: string }[]
  modelColumns: ColumnRow[]
  modelDimensions: SemanticModelDetail['dimensions']
  modelMetrics: SemanticModelDetail['metrics']
  t: LooseTFunction
}

const ExpressionNodeLiteral = memo(function ExpressionNodeLiteral({
  node,
  onChangeNode,
  t,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  t: LooseTFunction
}) {
  if (node.type !== 'literal') {
    return null
  }
  return (
    <div className={exprAstFlexRowClass}>
      <input
        type="text"
        className="input-text"
        placeholder={t('modeling.expr_literal_val_placeholder')}
        value={node.value !== null ? String(node.value) : ''}
        onChange={(e) => {
          const val = e.target.value
          const parsedNum = Number(val)
          onChangeNode({
            type: 'literal',
            value: isNaN(parsedNum) || val.trim() === '' ? val : parsedNum,
          })
        }}
      />
    </div>
  )
})

function ExpressionNodeColumnRef({
  node,
  onChangeNode,
  activeTables,
  modelColumns,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  activeTables: { schema: string; table: string }[]
  modelColumns: ColumnRow[]
}) {
  if (node.type !== 'column_ref') {
    return null
  }
  return (
    <div className={exprAstFlexRowClass}>
      <select
        className={exprAstSelectClass}
        value={node.table ?? ''}
        onChange={(e) => {
          onChangeNode({
            ...node,
            table: e.target.value,
          })
        }}
      >
        {activeTables.map((table) => (
          <option key={`${table.schema}.${table.table}`} value={table.table}>
            {table.table}
          </option>
        ))}
      </select>
      <select
        className={exprAstSelectClass}
        value={node.column || ''}
        onChange={(e) => {
          onChangeNode({
            ...node,
            column: e.target.value,
          })
        }}
      >
        {modelColumns
          .filter((c) => c.table_name === (node.table ?? activeTables[0]?.table ?? ''))
          .map((c) => (
            <option key={c.id} value={c.column_name}>
              {c.column_name} ({c.data_type})
            </option>
          ))}
      </select>
    </div>
  )
}

function ExpressionNodeDimensionRef({
  node,
  onChangeNode,
  modelDimensions,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  modelDimensions: SemanticModelDetail['dimensions']
}) {
  if (node.type !== 'dimension_ref') {
    return null
  }
  return (
    <div className={exprAstFlexRowClass}>
      <select
        className={exprAstSelectClass}
        value={node.name || ''}
        onChange={(e) => {
          onChangeNode({
            type: 'dimension_ref',
            name: e.target.value,
          })
        }}
      >
        {(modelDimensions ?? []).map((d) => (
          <option key={d.id} value={d.name}>
            {d.label ?? d.name}
          </option>
        ))}
      </select>
    </div>
  )
}

function ExpressionNodeMetricRef({
  node,
  onChangeNode,
  modelMetrics,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  modelMetrics: SemanticModelDetail['metrics']
}) {
  if (node.type !== 'metric_ref') {
    return null
  }
  return (
    <div className={exprAstFlexRowClass}>
      <select
        className={exprAstSelectClass}
        value={node.name || ''}
        onChange={(e) => {
          onChangeNode({
            type: 'metric_ref',
            name: e.target.value,
          })
        }}
      >
        {(modelMetrics ?? []).map((m) => (
          <option key={m.id} value={m.name}>
            {m.label ?? m.name}
          </option>
        ))}
      </select>
    </div>
  )
}

function ExpressionNodeBinary({
  node,
  onChangeNode,
  activeTables,
  modelColumns,
  modelDimensions,
  modelMetrics,
  t,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  activeTables: { schema: string; table: string }[]
  modelColumns: ColumnRow[]
  modelDimensions: SemanticModelDetail['dimensions']
  modelMetrics: SemanticModelDetail['metrics']
  t: LooseTFunction
}) {
  const nodeRef = useRef(node)
  // eslint-disable-next-line react-hooks/refs
  nodeRef.current = node

  const handleLeftChange = useCallback(
    (left: SemanticExprNode) => {
      onChangeNode({ ...nodeRef.current, left } as unknown as SemanticExprNode)
    },
    [onChangeNode],
  )

  const handleRightChange = useCallback(
    (right: SemanticExprNode) => {
      onChangeNode({ ...nodeRef.current, right } as unknown as SemanticExprNode)
    },
    [onChangeNode],
  )

  if (node.type !== 'binary') {
    return null
  }
  return (
    <div className="border-border-strong flex flex-col gap-3 border-l border-dashed pl-2">
      <div className="w-full">
        <ExpressionNodeBuilder
          node={node.left}
          onChangeNode={handleLeftChange}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
      </div>
      <div className="flex items-center py-1">
        <select
          className={exprAstOperatorSelectClass}
          value={node.op}
          onChange={(e) => onChangeNode({ ...node, op: e.target.value })}
        >
          {BINARY_OPS.map((op) => (
            <option key={op.value} value={op.value}>
              {op.label}
            </option>
          ))}
        </select>
      </div>
      <div className="w-full">
        <ExpressionNodeBuilder
          node={node.right}
          onChangeNode={handleRightChange}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
      </div>
    </div>
  )
}

function ExpressionNodeUnary({
  node,
  onChangeNode,
  activeTables,
  modelColumns,
  modelDimensions,
  modelMetrics,
  t,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  activeTables: { schema: string; table: string }[]
  modelColumns: ColumnRow[]
  modelDimensions: SemanticModelDetail['dimensions']
  modelMetrics: SemanticModelDetail['metrics']
  t: LooseTFunction
}) {
  const nodeRef = useRef(node)
  // eslint-disable-next-line react-hooks/refs
  nodeRef.current = node

  const handleExprChange = useCallback(
    (expr: SemanticExprNode) => {
      const current = nodeRef.current as SemanticUnaryExpr
      onChangeNode({ ...current, expr })
    },
    [onChangeNode],
  )

  if (node.type !== 'unary') {
    return null
  }
  return (
    <div className="flex items-center gap-3">
      <select
        className={exprAstOperatorSelectClass}
        value={node.op}
        onChange={(e) => onChangeNode({ ...node, op: e.target.value })}
      >
        {UNARY_OPS.map((op) => (
          <option key={op.value} value={op.value}>
            {op.label}
          </option>
        ))}
      </select>
      <div className="w-full">
        <ExpressionNodeBuilder
          node={node.expr}
          onChangeNode={handleExprChange}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
      </div>
    </div>
  )
}

function ExpressionNodeFunctionCall({
  node,
  onChangeNode,
  activeTables,
  modelColumns,
  modelDimensions,
  modelMetrics,
  t,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  activeTables: { schema: string; table: string }[]
  modelColumns: ColumnRow[]
  modelDimensions: SemanticModelDetail['dimensions']
  modelMetrics: SemanticModelDetail['metrics']
  t: LooseTFunction
}) {
  const nodeRef = useRef(node)
  // eslint-disable-next-line react-hooks/refs
  nodeRef.current = node

  const handleArgChange = useCallback(
    (idx: number) => (newArg: SemanticExprNode) => {
      const current = nodeRef.current as SemanticFunctionCallExpr
      const updated = [...(current.args ?? [])]
      updated[idx] = newArg
      onChangeNode({ ...current, args: updated })
    },
    [onChangeNode],
  )

  if (node.type !== 'function_call') {
    return null
  }
  return (
    <div>
      <div className="mb-2 flex items-center gap-3">
        <select
          className={exprAstSelectClass}
          value={node.name.toUpperCase()}
          onChange={(e) => {
            const func = ALLOWED_FUNCTIONS.find((f) => f.name === e.target.value)
            const argsCount = func ? (func.arity === -1 ? 1 : func.arity) : 1
            const newArgs: SemanticExprNode[] = Array.from({ length: argsCount }).map(
              (_, i) => node.args?.[i] ?? { type: 'literal', value: '' },
            )
            onChangeNode({
              type: 'function_call',
              name: e.target.value,
              args: newArgs,
            })
          }}
        >
          {ALLOWED_FUNCTIONS.map((f) => (
            <option key={f.name} value={f.name}>
              {f.name}
            </option>
          ))}
        </select>
        <span className="text-foreground-muted text-[0.72rem]">
          {ALLOWED_FUNCTIONS.find((f) => f.name.toUpperCase() === node.name.toUpperCase())?.desc}
        </span>
      </div>
      <div className="border-border-strong flex flex-col gap-3 border-l border-dashed pl-3">
        {(node.args ?? []).map((arg, idx) => (
          <div key={idx} className="flex items-center gap-2">
            <span className="text-foreground-muted text-[0.75rem] font-semibold whitespace-nowrap">
              Arg {idx + 1}:
            </span>
            <div className="w-full">
              <ExpressionNodeBuilder
                node={arg}
                onChangeNode={handleArgChange(idx)}
                activeTables={activeTables}
                modelColumns={modelColumns}
                modelDimensions={modelDimensions}
                modelMetrics={modelMetrics}
                t={t}
              />
            </div>
            {node.args && node.args.length > 1 && (
              <button
                type="button"
                className={exprAstRemoveBtnClass}
                onClick={() => {
                  onChangeNode({
                    ...node,
                    args: node.args?.filter((_, i) => i !== idx),
                  })
                }}
              >
                ×
              </button>
            )}
          </div>
        ))}
        {ALLOWED_FUNCTIONS.find((f) => f.name.toUpperCase() === node.name.toUpperCase())?.arity ===
          -1 && (
          <button
            type="button"
            className={exprAstAddBtnClass}
            onClick={() => {
              onChangeNode({
                ...node,
                args: [...(node.args ?? []), { type: 'literal', value: '' }],
              })
            }}
          >
            + {t('modeling.expr_add_arg')}
          </button>
        )}
      </div>
    </div>
  )
}

function ExpressionNodeCase({
  node,
  onChangeNode,
  activeTables,
  modelColumns,
  modelDimensions,
  modelMetrics,
  t,
}: {
  node: SemanticExprNode
  onChangeNode: (n: SemanticExprNode) => void
  activeTables: { schema: string; table: string }[]
  modelColumns: ColumnRow[]
  modelDimensions: SemanticModelDetail['dimensions']
  modelMetrics: SemanticModelDetail['metrics']
  t: LooseTFunction
}) {
  const nodeRef = useRef(node)
  // eslint-disable-next-line react-hooks/refs
  nodeRef.current = node

  const handleWhenChange = useCallback(
    (idx: number) => (when: SemanticExprNode) => {
      const current = nodeRef.current as SemanticCaseExpr
      const updated = [...(current.conditions ?? [])]
      updated[idx] = {
        when,
        then: updated[idx]?.then ?? { type: 'literal', value: '' },
      }
      onChangeNode({ ...current, conditions: updated })
    },
    [onChangeNode],
  )

  const handleThenChange = useCallback(
    (idx: number) => (then: SemanticExprNode) => {
      const current = nodeRef.current as SemanticCaseExpr
      const updated = [...(current.conditions ?? [])]
      updated[idx] = {
        when: updated[idx]?.when ?? { type: 'literal', value: '' },
        then,
      }
      onChangeNode({ ...current, conditions: updated })
    },
    [onChangeNode],
  )

  const handleElseChange = useCallback(
    (elseNode: SemanticExprNode) => {
      const current = nodeRef.current as SemanticCaseExpr
      onChangeNode({ ...current, else: elseNode })
    },
    [onChangeNode],
  )

  if (node.type !== 'case') {
    return null
  }
  return (
    <div>
      <div className="mb-4 flex flex-col gap-4">
        {(node.conditions ?? []).map((cond, idx) => (
          <div
            key={idx}
            className={`border-border relative flex flex-col gap-2 rounded-md border bg-white/1.5 p-3`}
          >
            <div className="flex items-center gap-3">
              <span className={exprAstCaseLabelClass}>WHEN</span>
              <ExpressionNodeBuilder
                node={cond.when}
                onChangeNode={handleWhenChange(idx)}
                activeTables={activeTables}
                modelColumns={modelColumns}
                modelDimensions={modelDimensions}
                modelMetrics={modelMetrics}
                t={t}
              />
            </div>
            <div className="flex items-center gap-3">
              <span className={exprAstCaseLabelClass}>THEN</span>
              <ExpressionNodeBuilder
                node={cond.then}
                onChangeNode={handleThenChange(idx)}
                activeTables={activeTables}
                modelColumns={modelColumns}
                modelDimensions={modelDimensions}
                modelMetrics={modelMetrics}
                t={t}
              />
            </div>
            {node.conditions && node.conditions.length > 1 && (
              <button
                type="button"
                className={exprAstRemoveBtnClass}
                onClick={() => {
                  onChangeNode({
                    ...node,
                    conditions: node.conditions?.filter((_, i) => i !== idx),
                  })
                }}
              >
                ×
              </button>
            )}
          </div>
        ))}
        <button
          type="button"
          className={exprAstAddBtnClass}
          onClick={() => {
            onChangeNode({
              ...node,
              conditions: [
                ...(node.conditions ?? []),
                {
                  when: { type: 'literal', value: true },
                  then: { type: 'literal', value: '' },
                },
              ],
            })
          }}
        >
          + {t('modeling.expr_add_when')}
        </button>
      </div>

      <div className={`border-border flex items-center gap-3 border-t border-dashed pt-3`}>
        <span className={exprAstCaseLabelClass}>ELSE</span>
        <ExpressionNodeBuilder
          node={node.else ?? { type: 'literal', value: '' }}
          onChangeNode={handleElseChange}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
      </div>
    </div>
  )
}

const ExpressionNodeBuilder = memo(function ExpressionNodeBuilder({
  node,
  onChangeNode,
  activeTables,
  modelColumns,
  modelDimensions,
  modelMetrics,
  t,
}: ExpressionNodeBuilderProps) {
  const typeSelectId = useId()

  const handleTypeChange = (newType: string) => {
    onChangeNode(getDefaultNode(newType, activeTables, modelColumns, modelDimensions, modelMetrics))
  }

  return (
    <div className={exprAstNodeCardClass}>
      <div>
        <select
          id={typeSelectId}
          className={exprAstTypeSelectClass}
          value={node.type}
          onChange={(e) => handleTypeChange(e.target.value)}
        >
          <option value="literal">{t('modeling.expr_literal')}</option>
          <option value="column_ref">{t('modeling.expr_column_ref')}</option>
          <option value="dimension_ref">{t('modeling.expr_dimension_ref')}</option>
          <option value="metric_ref">{t('modeling.expr_metric_ref')}</option>
          <option value="binary">{t('modeling.expr_binary')}</option>
          <option value="unary">{t('modeling.expr_unary')}</option>
          <option value="function_call">{t('modeling.expr_function')}</option>
          <option value="case">{t('modeling.expr_case')}</option>
        </select>
      </div>

      <div>
        <ExpressionNodeLiteral node={node} onChangeNode={onChangeNode} t={t} />
        <ExpressionNodeColumnRef
          node={node}
          onChangeNode={onChangeNode}
          activeTables={activeTables}
          modelColumns={modelColumns}
        />
        <ExpressionNodeDimensionRef
          node={node}
          onChangeNode={onChangeNode}
          modelDimensions={modelDimensions}
        />
        <ExpressionNodeMetricRef
          node={node}
          onChangeNode={onChangeNode}
          modelMetrics={modelMetrics}
        />
        <ExpressionNodeBinary
          node={node}
          onChangeNode={onChangeNode}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
        <ExpressionNodeUnary
          node={node}
          onChangeNode={onChangeNode}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
        <ExpressionNodeFunctionCall
          node={node}
          onChangeNode={onChangeNode}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
        <ExpressionNodeCase
          node={node}
          onChangeNode={onChangeNode}
          activeTables={activeTables}
          modelColumns={modelColumns}
          modelDimensions={modelDimensions}
          modelMetrics={modelMetrics}
          t={t}
        />
      </div>
    </div>
  )
})
