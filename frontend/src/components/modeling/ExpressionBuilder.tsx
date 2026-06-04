import { useEffect, useState, useCallback, useMemo } from 'react'
import type {
  SemanticModelDetail,
  SemanticExprNode,
  ColumnRow,
  TableRow,
} from '../../types/semantic'

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

interface ExpressionBuilderProps {
  model: SemanticModelDetail
  columns: ColumnRow[]
  initialNode?: SemanticExprNode
  initialText?: string
  onChange: (node: SemanticExprNode, textExpression: string) => void
  t: (key: string, vars?: any) => string
}

export function ExpressionBuilder({
  model,
  columns,
  initialNode,
  initialText = '',
  onChange,
  t,
}: ExpressionBuilderProps) {
  const [mode, setMode] = useState<'visual' | 'text'>('text')
  const [textInput, setTextInput] = useState(initialText)
  const [astNode, setAstNode] = useState<SemanticExprNode>(
    initialNode || { type: 'literal', value: '' }
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
        keys.add(`${j.from_schema || model.base_schema}.${j.from_table}`)
        keys.add(`${j.to_schema || model.base_schema}.${j.to_table}`)
      }
    })
    return Array.from(keys).map((k) => {
      const parts = k.split('.')
      return { schema: parts[0], table: parts[1] }
    })
  }, [model])

  // Filter columns based on active tables
  const modelColumns = useMemo(() => {
    return columns.filter((col) =>
      activeTables.some(
        (t) => t.schema === col.schema_name && t.table === col.table_name
      )
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
        const res = await fetch(
          `/api/semantic/models/${model.id}/compile-expression`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          }
        )
        const data = await res.json()
        if (res.ok) {
          setCompiledSQL(data.sql || '')
          setErrorMsg(null)
          if (data.expr) {
            setAstNode(data.expr)
          }
          // Notify parent of latest valid state
          onChange(data.expr || astNode, payload.expression || data.sql || '')
        } else {
          setErrorMsg(data.error || 'Failed to compile expression')
          setCompiledSQL('')
        }
      } catch (err: any) {
        setErrorMsg('Network error: ' + err.message)
        setCompiledSQL('')
      } finally {
        setLoading(false)
      }
    },
    [model.id, onChange, astNode]
  )

  // Trigger compile when Visual AST changes
  const handleAstChange = useCallback(
    (newNode: SemanticExprNode) => {
      setAstNode(newNode)
      void compileExpression({ expr: newNode })
    },
    [compileExpression]
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

  // Recursive AST node editor
  function ExpressionNodeBuilder({
    node,
    onChangeNode,
  }: {
    node: SemanticExprNode
    onChangeNode: (n: SemanticExprNode) => void
  }) {
    const handleTypeChange = (newType: string) => {
      let defaultValue: SemanticExprNode
      switch (newType) {
        case 'literal':
          defaultValue = { type: 'literal', value: '' }
          break
        case 'column_ref':
          defaultValue = {
            type: 'column_ref',
            table: model.base_table,
            column: modelColumns[0]?.column_name || '',
          }
          break
        case 'metric_ref':
          defaultValue = {
            type: 'metric_ref',
            name: modelMetrics[0]?.name || '',
          }
          break
        case 'dimension_ref':
          defaultValue = {
            type: 'dimension_ref',
            name: modelDimensions[0]?.name || '',
          }
          break
        case 'binary':
          defaultValue = {
            type: 'binary',
            op: 'add',
            left: { type: 'literal', value: 0 },
            right: { type: 'literal', value: 0 },
          }
          break
        case 'unary':
          defaultValue = {
            type: 'unary',
            op: 'not',
            expr: { type: 'literal', value: true },
          }
          break
        case 'function_call':
          defaultValue = {
            type: 'function_call',
            name: 'UPPER',
            args: [{ type: 'literal', value: '' }],
          }
          break
        case 'case':
          defaultValue = {
            type: 'case',
            conditions: [
              {
                when: { type: 'literal', value: true },
                then: { type: 'literal', value: '' },
              },
            ],
            else: { type: 'literal', value: '' },
          }
          break
        default:
          defaultValue = { type: 'literal', value: '' }
      }
      onChangeNode(defaultValue)
    }

    return (
      <div className="ast-node-card">
        <div className="ast-node-header">
          <select
            id={`node-type-select-${Math.random()}`}
            className="ast-type-select"
            value={node.type}
            onChange={(e) => handleTypeChange(e.target.value)}
          >
            <option value="literal">{t('modeling.expr_literal', 'Literal (Constant)')}</option>
            <option value="column_ref">{t('modeling.expr_column_ref', 'Table Column')}</option>
            <option value="dimension_ref">{t('modeling.expr_dimension_ref', 'Dimension Reference')}</option>
            <option value="metric_ref">{t('modeling.expr_metric_ref', 'Metric Reference')}</option>
            <option value="binary">{t('modeling.expr_binary', 'Math / Comparison Operator')}</option>
            <option value="unary">{t('modeling.expr_unary', 'Unary Operator (NOT / Negate)')}</option>
            <option value="function_call">{t('modeling.expr_function', 'Function Call')}</option>
            <option value="case">{t('modeling.expr_case', 'Case Expression (Conditional)')}</option>
          </select>
        </div>

        <div className="ast-node-body">
          {node.type === 'literal' && (
            <div className="ast-literal-editor">
              <input
                type="text"
                className="input-text"
                placeholder={t('modeling.expr_literal_val_placeholder', 'Enter value...')}
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
          )}

          {node.type === 'column_ref' && (
            <div className="ast-column-editor">
              <select
                className="ast-select"
                value={node.table || ''}
                onChange={(e) => {
                  onChangeNode({
                    ...node,
                    table: e.target.value,
                  })
                }}
              >
                {activeTables.map((t) => (
                  <option key={`${t.schema}.${t.table}`} value={t.table}>
                    {t.table}
                  </option>
                ))}
              </select>
              <select
                className="ast-select"
                value={node.column || ''}
                onChange={(e) => {
                  onChangeNode({
                    ...node,
                    column: e.target.value,
                  })
                }}
              >
                {modelColumns
                  .filter((c) => c.table_name === (node.table || model.base_table))
                  .map((c) => (
                    <option key={c.id} value={c.column_name}>
                      {c.column_name} ({c.data_type})
                    </option>
                  ))}
              </select>
            </div>
          )}

          {node.type === 'dimension_ref' && (
            <div className="ast-ref-editor">
              <select
                className="ast-select"
                value={node.name || ''}
                onChange={(e) => {
                  onChangeNode({
                    type: 'dimension_ref',
                    name: e.target.value,
                  })
                }}
              >
                {modelDimensions.map((d) => (
                  <option key={d.id} value={d.name}>
                    {d.label || d.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {node.type === 'metric_ref' && (
            <div className="ast-ref-editor">
              <select
                className="ast-select"
                value={node.name || ''}
                onChange={(e) => {
                  onChangeNode({
                    type: 'metric_ref',
                    name: e.target.value,
                  })
                }}
              >
                {modelMetrics.map((m) => (
                  <option key={m.id} value={m.name}>
                    {m.label || m.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {node.type === 'binary' && (
            <div className="ast-binary-editor">
              <div className="ast-sub-expr">
                <ExpressionNodeBuilder
                  node={node.left}
                  onChangeNode={(left) => onChangeNode({ ...node, left })}
                />
              </div>
              <div className="ast-operator-select-container">
                <select
                  className="ast-operator-select"
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
              <div className="ast-sub-expr">
                <ExpressionNodeBuilder
                  node={node.right}
                  onChangeNode={(right) => onChangeNode({ ...node, right })}
                />
              </div>
            </div>
          )}

          {node.type === 'unary' && (
            <div className="ast-unary-editor">
              <select
                className="ast-operator-select"
                value={node.op}
                onChange={(e) => onChangeNode({ ...node, op: e.target.value })}
              >
                {UNARY_OPS.map((op) => (
                  <option key={op.value} value={op.value}>
                    {op.label}
                  </option>
                ))}
              </select>
              <div className="ast-sub-expr">
                <ExpressionNodeBuilder
                  node={node.expr}
                  onChangeNode={(expr) => onChangeNode({ ...node, expr })}
                />
              </div>
            </div>
          )}

          {node.type === 'function_call' && (
            <div className="ast-function-editor">
              <div className="ast-func-header">
                <select
                  className="ast-select"
                  value={node.name.toUpperCase()}
                  onChange={(e) => {
                    const func = ALLOWED_FUNCTIONS.find(
                      (f) => f.name === e.target.value
                    )
                    const argsCount = func ? (func.arity === -1 ? 1 : func.arity) : 1
                    const newArgs: SemanticExprNode[] = Array.from({ length: argsCount }).map(
                      (_, i) => node.args?.[i] || { type: 'literal', value: '' }
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
                <span className="ast-func-desc">
                  {ALLOWED_FUNCTIONS.find(
                    (f) => f.name.toUpperCase() === node.name.toUpperCase()
                  )?.desc}
                </span>
              </div>
              <div className="ast-func-args">
                {(node.args || []).map((arg, idx) => (
                  <div key={idx} className="ast-func-arg-row">
                    <span className="ast-arg-label">Arg {idx + 1}:</span>
                    <div className="ast-sub-expr">
                      <ExpressionNodeBuilder
                        node={arg}
                        onChangeNode={(newArg) => {
                          const updated = [...(node.args || [])]
                          updated[idx] = newArg
                          onChangeNode({ ...node, args: updated })
                        }}
                      />
                    </div>
                    {node.args && node.args.length > 1 && (
                      <button
                        type="button"
                        className="ast-remove-btn"
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
                {ALLOWED_FUNCTIONS.find(
                  (f) => f.name.toUpperCase() === node.name.toUpperCase()
                )?.arity === -1 && (
                  <button
                    type="button"
                    className="ast-add-btn"
                    onClick={() => {
                      onChangeNode({
                        ...node,
                        args: [
                          ...(node.args || []),
                          { type: 'literal', value: '' },
                        ],
                      })
                    }}
                  >
                    + {t('modeling.expr_add_arg', 'Add Argument')}
                  </button>
                )}
              </div>
            </div>
          )}

          {node.type === 'case' && (
            <div className="ast-case-editor">
              <div className="ast-case-conditions">
                {(node.conditions || []).map((cond, idx) => (
                  <div key={idx} className="ast-case-cond-row">
                    <div className="ast-case-cond-block">
                      <span className="ast-case-label">WHEN</span>
                      <ExpressionNodeBuilder
                        node={cond.when}
                        onChangeNode={(when) => {
                          const updated = [...(node.conditions || [])]
                          updated[idx] = { when, then: updated[idx]?.then || { type: 'literal', value: '' } }
                          onChangeNode({ ...node, conditions: updated })
                        }}
                      />
                    </div>
                    <div className="ast-case-cond-block">
                      <span className="ast-case-label">THEN</span>
                      <ExpressionNodeBuilder
                        node={cond.then}
                        onChangeNode={(then) => {
                          const updated = [...(node.conditions || [])]
                          updated[idx] = { when: updated[idx]?.when || { type: 'literal', value: '' }, then }
                          onChangeNode({ ...node, conditions: updated })
                        }}
                      />
                    </div>
                    {node.conditions && node.conditions.length > 1 && (
                      <button
                        type="button"
                        className="ast-remove-btn"
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
                  className="ast-add-btn"
                  onClick={() => {
                    onChangeNode({
                      ...node,
                      conditions: [
                        ...(node.conditions || []),
                        {
                          when: { type: 'literal', value: true },
                          then: { type: 'literal', value: '' },
                        },
                      ],
                    })
                  }}
                >
                  + {t('modeling.expr_add_when', 'Add WHEN Condition')}
                </button>
              </div>

              <div className="ast-case-else">
                <span className="ast-case-label">ELSE</span>
                <ExpressionNodeBuilder
                  node={node.else || { type: 'literal', value: '' }}
                  onChangeNode={(elseNode) =>
                    onChangeNode({ ...node, else: elseNode })
                  }
                />
              </div>
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="expression-builder-panel">
      <div className="expression-builder-header">
        <button
          type="button"
          className={`toggle-btn ${mode === 'text' ? 'active' : ''}`}
          onClick={toggleMode}
        >
          {t('modeling.expr_mode_text', 'Advanced Text Mode')}
        </button>
        <button
          type="button"
          className={`toggle-btn ${mode === 'visual' ? 'active' : ''}`}
          onClick={toggleMode}
        >
          {t('modeling.expr_mode_visual', 'Visual Tree Mode')}
        </button>
      </div>

      <div className="expression-builder-body">
        {mode === 'visual' ? (
          <div className="ast-tree-root">
            <ExpressionNodeBuilder
              node={astNode}
              onChangeNode={handleAstChange}
            />
          </div>
        ) : (
          <div className="text-editor-wrapper">
            <textarea
              id="raw-text-expression"
              className="expression-editor-textarea expression-editor-textarea--visible"
              value={textInput}
              onChange={(e) => setTextInput(e.target.value)}
              placeholder="e.g. sum([orders.total_amount]) - sum([orders.discount])"
              rows={4}
              disabled={loading}
            />
            <div className="editor-intellisense-help">
              {t('modeling.metric_intellisense_hint', 'Type [ or a letter for autocomplete')}
            </div>
          </div>
        )}
      </div>

      {errorMsg && <div className="expression-builder-error">{errorMsg}</div>}

      <div className="expression-builder-preview">
        <h4>{t('modeling.generated_sql', 'Generated SQL Preview')}</h4>
        {loading ? (
          <div className="preview-loading">{t('modeling.running', 'Compiling…')}</div>
        ) : (
          <code className="sql-preview-code">
            {compiledSQL || '-- Type an expression or build one visually to see SQL --'}
          </code>
        )}
      </div>
    </div>
  )
}
