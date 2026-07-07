# AI Chat Follow-Up Suggestions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make AI chat answers feel like a useful BI agent by adding richer answer explanations and context-aware follow-up question choices under each successful response.

**Architecture:** The backend owns durable, structured follow-up suggestions when an AI response is produced, validates them against existing schema/semantic context, and persists them in the response payload. The frontend renders those suggestions as compact next-action controls inside the assistant message, with a deterministic frontend fallback when older responses or partial backend responses do not include suggestions. The first shipped interaction fills and focuses the composer instead of auto-running a query, preserving user control.

**Tech Stack:** Go 1.26 backend, existing AI query/job response flow, React 19 + TypeScript + Vite 6 frontend, existing `AIQueryResponse` / `ConversationMessage` contracts, `useT()` i18n, Tailwind utility classes for new UI surfaces, Vitest, `make check-frontend`, `make lint-go`, `make test-go`.

## Global Constraints

- Do not generate follow-up questions that reference fields, tables, metrics, dimensions, or dates that are not present in the current semantic/query context.
- Show at most 3 follow-up suggestions per assistant response.
- Do not show duplicate suggestions that match prior user questions in the active conversation.
- Follow-up buttons must be accessible `button` elements with keyboard focus states and localized labels.
- Backend AI-generated suggestions must be validated before they are returned or persisted.
- Frontend fallback suggestions must be deterministic and safe; they may be generic, but they must not invent domain fields.
- The first implementation must fill the chat composer and focus it; auto-submit can be a later explicit feature.
- Keep existing answer, details, result table/chart, feedback, and conversation persistence behavior intact.
- Use `errors.Is`, `errors.AsType[T]`, `new(expr)`, `for i := range n`, `min` / `max`, and `slices.Contains` where they apply to touched Go code.

---

## Desired User Experience

For a user question like:

```text
24 saatlik dilimlere gore hangi saat diliminde daha cok tweet atilmis dun?
```

The assistant should feel more like an agent:

```text
Saat 17:00 diliminde 66 tweet atilmis.

Bu sonuc, created_at_ts alaninin saat bazinda gruplanmasiyla hesaplandi.
24 saatlik dagilim icinde en yuksek deger 17:00 diliminde gorunuyor.

Devam etmek ister misin?
[Bu saati gunlere gore kir] [En yogun 5 saati karsilastir] [Saatlik dagilimi grafikte goster]
```

The visible chip labels stay short. The hidden/full question sent to the composer can be more explicit:

```text
Dunku tweetleri saat bazinda gruplayip en yogun 5 saat dilimini karsilastir.
```

---

## File Structure

### Backend

- Modify the existing Go AI response contract file that defines the response returned to the frontend. During implementation, locate it with `gograph_query` / `gograph_context` rather than text-searching Go symbols.
- Modify the existing AI query/job assembly path that builds the final response payload persisted into agent run/job result JSON.
- Create a focused backend helper near the AI response assembly code:
  - Responsibility: validate, deduplicate, cap, and fallback follow-up suggestions.
  - Suggested name: `followup_suggestions.go` inside the same Go package as the AI response assembly.
- Add backend tests in the matching package:
  - Validate accepted structured suggestions.
  - Drop invalid suggestions.
  - Drop duplicate/history-similar suggestions.
  - Cap output at 3.
  - Produce deterministic fallback when AI suggestions are empty or rejected.

### Frontend

- Modify: `frontend/src/types/ai.ts`
  - Add `SuggestedFollowUp` and `suggested_followups?: SuggestedFollowUp[]`.
- Create: `frontend/src/components/aiQuery/followUpSuggestions.ts`
  - Deterministic frontend fallback generation and duplicate filtering.
- Create: `frontend/src/components/aiQuery/FollowUpSuggestions.tsx`
  - UI component for rendering suggestion chips.
- Modify: `frontend/src/components/aiQuery/AssistantMessageCard.tsx`
  - Render suggestions after answer/results and before feedback.
- Modify: `frontend/src/components/AIQuery.tsx`
  - Pass `onSelectFollowUp` to assistant cards and focus the composer after filling it.
- Modify: `frontend/src/components/aiQuery/types.ts`
  - Add callback props needed by `AssistantMessageCard`.
- Modify: `frontend/src/components/aiQuery/resultInsight.ts`
  - Expand deterministic answer captions to 1-2 useful explanatory sentences.
- Modify: `frontend/src/i18n/locales/tr/core.ts`
  - Add Turkish follow-up and explanation keys.
- Modify: `frontend/src/i18n/locales/en/core.ts`
  - Add English follow-up and explanation keys.
- Add tests:
  - `frontend/src/components/aiQuery/followUpSuggestions.test.ts`
  - Existing component tests or a focused test for follow-up rendering and click behavior, depending on current test setup.

---

## Data Contract

### TypeScript Contract

Add this to `frontend/src/types/ai.ts`:

```ts
export type SuggestedFollowUpKind =
  | 'breakdown'
  | 'comparison'
  | 'trend'
  | 'chart'
  | 'drilldown'
  | 'filter'
  | 'explain'

export interface SuggestedFollowUp {
  id: string
  label: string
  question: string
  reason?: string
  kind: SuggestedFollowUpKind
  requires?: string[]
}
```

Extend `AIQueryResponse`:

```ts
suggested_followups?: SuggestedFollowUp[]
```

### Go Contract

Add the equivalent Go types in the existing backend AI response contract package:

```go
type SuggestedFollowUpKind string

const (
	SuggestedFollowUpBreakdown  SuggestedFollowUpKind = "breakdown"
	SuggestedFollowUpComparison SuggestedFollowUpKind = "comparison"
	SuggestedFollowUpTrend      SuggestedFollowUpKind = "trend"
	SuggestedFollowUpChart      SuggestedFollowUpKind = "chart"
	SuggestedFollowUpDrilldown  SuggestedFollowUpKind = "drilldown"
	SuggestedFollowUpFilter     SuggestedFollowUpKind = "filter"
	SuggestedFollowUpExplain    SuggestedFollowUpKind = "explain"
)

type SuggestedFollowUp struct {
	ID       string                `json:"id"`
	Label    string                `json:"label"`
	Question string                `json:"question"`
	Reason   string                `json:"reason,omitempty"`
	Kind     SuggestedFollowUpKind `json:"kind"`
	Requires []string              `json:"requires,omitempty"`
}
```

Extend the existing response struct:

```go
SuggestedFollowUps []SuggestedFollowUp `json:"suggested_followups,omitempty"`
```

---

## Backend Suggestion Rules

Backend suggestion generation should use a hybrid model:

1. Deterministic candidate builder creates safe candidate intents from the executed result shape.
2. AI may rewrite, rank, or select from those candidates when enough context exists.
3. Backend validation is the final authority.
4. Deterministic fallback returns safe options when AI output is empty, invalid, or unavailable.

### AI Prompt Contract

The AI follow-up prompt must ask for strict JSON only:

```json
{
  "suggestions": [
    {
      "id": "top-hours",
      "label": "En yogun 5 saati karsilastir",
      "question": "Dunku tweetleri saat bazinda gruplayip en yogun 5 saat dilimini karsilastir.",
      "reason": "Current result found a single busiest hour.",
      "kind": "comparison",
      "requires": ["created_at_ts"]
    }
  ]
}
```

Prompt constraints:

- Return at most 3 suggestions.
- Use only fields, tables, dimensions, and metrics listed in the request context.
- Prefer questions that can be answered by the current datasource and semantic model.
- Do not repeat any prior user question.
- Do not include SQL.
- Do not include markdown.
- If no safe suggestion exists, return `{"suggestions":[]}`.

### Validation Rules

Validate every AI-generated suggestion before returning it:

- Trim `id`, `label`, `question`, `reason`, and each `requires` value.
- Drop suggestion when `label` or `question` is empty.
- Drop suggestion when `kind` is not in the allowlist.
- Drop suggestion when `requires` references a field not present in the current query/semantic context.
- Drop suggestion when normalized `question` is too similar to a prior user question.
- Drop suggestion when normalized `label` duplicates another suggestion.
- Cap to 3 suggestions after filtering.

Normalization function requirements:

```go
func normalizeSuggestionText(s string) string {
	// Lowercase, trim, collapse whitespace, remove simple punctuation.
}
```

Similarity MVP:

- Exact normalized match is duplicate.
- `strings.Contains(prior, candidate)` or `strings.Contains(candidate, prior)` is duplicate when the shorter side has length >= 16.

---

## Frontend Interaction Model

### Layout

Inside each successful assistant message:

1. Existing assistant answer.
2. Expanded deterministic insight/caption.
3. Existing chart/table/results.
4. New follow-up suggestion chips.
5. Existing feedback section.
6. Existing details toggle/content.

### Component API

`FollowUpSuggestions.tsx`:

```tsx
import type { TFunction } from '../../i18n'
import type { SuggestedFollowUp } from '../../types/ai'

type FollowUpSuggestionsProps = {
  suggestions: SuggestedFollowUp[]
  onSelect: (question: string) => void
  t: TFunction
}

export function FollowUpSuggestions({ suggestions, onSelect, t }: FollowUpSuggestionsProps) {
  if (suggestions.length === 0) {
    return null
  }

  return (
    <section className="mt-4 border-t border-border/70 pt-3" aria-label={t('ai_query.followups_title')}>
      <p className="mb-2 text-sm font-medium text-text">{t('ai_query.followups_title')}</p>
      <div className="flex flex-wrap gap-2">
        {suggestions.map((suggestion) => (
          <button
            key={suggestion.id}
            type="button"
            className="rounded-md border border-border bg-surface-2 px-3 py-1.5 text-sm text-text transition hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            aria-label={t('ai_query.followups_apply_aria', { question: suggestion.question })}
            onClick={() => onSelect(suggestion.question)}
          >
            {suggestion.label}
          </button>
        ))}
      </div>
    </section>
  )
}
```

### Composer Behavior

In `AIQuery.tsx`, add a textarea ref if one does not already exist:

```tsx
const questionInputRef = useRef<HTMLTextAreaElement | null>(null)

const handleSelectFollowUp = useCallback((nextQuestion: string) => {
  setQuestion(nextQuestion)
  window.requestAnimationFrame(() => {
    questionInputRef.current?.focus()
  })
}, [])
```

Pass `questionInputRef` to the composer textarea and pass `handleSelectFollowUp` down to `AssistantMessageCard`.

Do not auto-submit in the first implementation. The user should be able to edit the generated follow-up question before sending.

---

## Task 1: Backend Contract and Validation

**Files:**

- Modify: existing Go AI response contract file found with `gograph_query SuggestedFollowUp` / `gograph_query Response`.
- Create or modify: backend AI response assembly package file `followup_suggestions.go`.
- Test: matching `followup_suggestions_test.go`.

**Interfaces:**

- Produces: `SuggestedFollowUp`, `SuggestedFollowUpKind`, `ValidateSuggestedFollowUps`.
- Consumes: current response type and prior user question list.

- [x] **Step 1: Run gograph preflight**

Run:

```bash
gograph build .
```

Then use MCP:

```text
gograph_query "Response"
gograph_plan "<actual response symbol>"
```

Expected: identify the backend response struct and the response assembly path before editing.

- [x] **Step 2: Add backend types**

Add the Go contract from the "Go Contract" section to the existing response contract package.

- [x] **Step 3: Add validation helper**

Implement:

```go
const maxSuggestedFollowUps = 3

func ValidateSuggestedFollowUps(
	candidates []SuggestedFollowUp,
	availableFields []string,
	priorQuestions []string,
) []SuggestedFollowUp {
	// 1. Build available field set.
	// 2. Normalize prior questions.
	// 3. Iterate candidates in order.
	// 4. Trim fields.
	// 5. Validate kind.
	// 6. Validate requires fields.
	// 7. Drop duplicates.
	// 8. Append until maxSuggestedFollowUps.
}
```

The implementation must use `slices.Contains` or a map set for membership, not a hand-rolled repeated linear scan on hot paths.

- [x] **Step 4: Add tests for validation**

Test cases:

```go
func TestValidateSuggestedFollowUpsKeepsValidSuggestions(t *testing.T)
func TestValidateSuggestedFollowUpsDropsUnknownKind(t *testing.T)
func TestValidateSuggestedFollowUpsDropsUnknownRequiredField(t *testing.T)
func TestValidateSuggestedFollowUpsDropsPriorQuestionDuplicate(t *testing.T)
func TestValidateSuggestedFollowUpsCapsAtThree(t *testing.T)
```

- [x] **Step 5: Verify**

Run focused tests for the package:

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./<actual/package/path>
```

Expected: all new validation tests pass.

---

## Task 2: Backend Suggestion Generation and Persistence

**Files:**

- Modify: existing AI query/job response assembly code.
- Modify: existing AI job persistence path only if the response payload is not already persisted as full JSON.
- Test: matching handler/job assembly tests.

**Interfaces:**

- Consumes: `ValidateSuggestedFollowUps`.
- Produces: `suggested_followups` in final persisted `AIQueryResponse`.

- [x] **Step 1: Build deterministic candidates**

Create a pure helper near response assembly:

```go
func BuildDeterministicFollowUps(ctx FollowUpContext) []SuggestedFollowUp
```

Where `FollowUpContext` contains:

```go
type FollowUpContext struct {
	UserQuestion    string
	PriorQuestions  []string
	AvailableFields []string
	ResultColumns   []string
	ResultRowCount   int
	HasMetric       bool
	HasDimension    bool
	HasTimeField    bool
}
```

Candidate rules:

- `HasTimeField && HasMetric`: add trend/chart suggestion.
- `HasDimension && HasMetric && ResultRowCount > 1`: add top comparison suggestion.
- `ResultRowCount == 1 && HasMetric`: add drilldown/breakdown suggestion.
- Always validate before returning.

- [x] **Step 2: Add optional AI rewrite/select phase**

If the existing AI pipeline has a structured-output helper, use it. If it does not, keep Task 2 deterministic and leave AI rewrite as Task 8 below.

The AI phase must never bypass validation:

```go
raw := BuildDeterministicFollowUps(ctx)
rewritten, err := RewriteFollowUpsWithAI(ctx, raw)
if err != nil || len(rewritten) == 0 {
	return ValidateSuggestedFollowUps(raw, ctx.AvailableFields, ctx.PriorQuestions)
}
return ValidateSuggestedFollowUps(rewritten, ctx.AvailableFields, ctx.PriorQuestions)
```

- [x] **Step 3: Attach suggestions to response**

When assembling the final AI response, set:

```go
resp.SuggestedFollowUps = BuildFollowUpsForResponse(...)
```

Ensure this occurs before the response is serialized into any job/agent-run result JSON.

- [x] **Step 4: Verify persistence**

Add or update a test that stores a completed AI response and reloads it. Expected: `suggested_followups` survives round-trip.

- [x] **Step 5: Verify**

Run:

```bash
gofmt -w <touched-go-files>
GOCACHE=/private/tmp/biqly-gocache go test ./<touched/backend/package>/...
```

Expected: tests pass and response JSON includes `suggested_followups`.

---

## Task 3: Frontend Types and Fallback Suggestion Helper

**Files:**

- Modify: `frontend/src/types/ai.ts`
- Create: `frontend/src/components/aiQuery/followUpSuggestions.ts`
- Test: `frontend/src/components/aiQuery/followUpSuggestions.test.ts`

**Interfaces:**

- Produces: `normalizeFollowUpText`, `filterFollowUpSuggestions`, `buildFallbackFollowUps`.
- Consumes: `SuggestedFollowUp`, `AIQueryResponse`, `ConversationMessage`.

- [x] **Step 1: Add TypeScript types**

Add the TypeScript contract from the "TypeScript Contract" section.

- [x] **Step 2: Implement normalization**

```ts
export function normalizeFollowUpText(value: string): string {
  return value
    .toLocaleLowerCase('tr')
    .replace(/[^\p{L}\p{N}\s]/gu, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}
```

- [x] **Step 3: Implement duplicate filtering**

```ts
export function filterFollowUpSuggestions(
  suggestions: SuggestedFollowUp[],
  priorQuestions: string[],
): SuggestedFollowUp[] {
  const prior = priorQuestions.map(normalizeFollowUpText).filter(Boolean)
  const seen = new Set<string>()
  const kept: SuggestedFollowUp[] = []

  for (const suggestion of suggestions) {
    const label = suggestion.label.trim()
    const question = suggestion.question.trim()
    const normalized = normalizeFollowUpText(question)
    if (!label || !question || !normalized || seen.has(normalized)) {
      continue
    }
    if (prior.some((item) => item === normalized || item.includes(normalized) || normalized.includes(item))) {
      continue
    }
    seen.add(normalized)
    kept.push({ ...suggestion, label, question })
    if (kept.length >= 3) {
      break
    }
  }

  return kept
}
```

- [x] **Step 4: Implement fallback builder**

```ts
export function buildFallbackFollowUps(args: {
  response: AIQueryResponse
  priorQuestions: string[]
  t: TFunction
}): SuggestedFollowUp[] {
  const result = args.response.result
  if (!result || result.rows.length === 0) {
    return []
  }

  const hasMetric = result.columns.some((column) => column.semantic_type === 'metric')
  const hasDimension = result.columns.some((column) => column.semantic_type === 'dimension')
  const hasTime = result.columns.some((column) => /date|time|hour|day|month|year|_ts$/i.test(column.name))

  const candidates: SuggestedFollowUp[] = []
  if (hasTime && hasMetric) {
    candidates.push({
      id: 'fallback-trend',
      kind: 'trend',
      label: args.t('ai_query.followups_trend_label'),
      question: args.t('ai_query.followups_trend_question'),
    })
  }
  if (hasDimension && hasMetric && result.rows.length > 1) {
    candidates.push({
      id: 'fallback-compare',
      kind: 'comparison',
      label: args.t('ai_query.followups_compare_label'),
      question: args.t('ai_query.followups_compare_question'),
    })
  }
  if (hasMetric) {
    candidates.push({
      id: 'fallback-chart',
      kind: 'chart',
      label: args.t('ai_query.followups_chart_label'),
      question: args.t('ai_query.followups_chart_question'),
    })
  }

  return filterFollowUpSuggestions(candidates, args.priorQuestions)
}
```

- [x] **Step 5: Add tests**

Tests:

```ts
it('keeps backend suggestions and caps them at three')
it('drops suggestions matching prior user questions')
it('builds chart fallback for metric results')
it('returns no fallback for empty results')
```

- [x] **Step 6: Verify**

Run:

```bash
npm --prefix frontend run test -- followUpSuggestions
```

Expected: focused tests pass.

---

## Task 4: Frontend Follow-Up UI Component

**Files:**

- Create: `frontend/src/components/aiQuery/FollowUpSuggestions.tsx`
- Test: component test if the repository has an existing React Testing Library setup; otherwise cover behavior through parent component tests in Task 5.

**Interfaces:**

- Consumes: `SuggestedFollowUp[]`, `onSelect(question)`, `t`.
- Produces: accessible suggestion buttons.

- [x] **Step 1: Implement component**

Use the component API from "Frontend Interaction Model".

- [x] **Step 2: Keep visual style compact**

Use Tailwind utilities:

```tsx
className="mt-4 border-t border-border/70 pt-3"
className="mb-2 text-sm font-medium text-text"
className="flex flex-wrap gap-2"
className="rounded-md border border-border bg-surface-2 px-3 py-1.5 text-sm text-text transition hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
```

- [x] **Step 3: Accessibility checks**

Each button must:

- use `type="button"`
- have visible text equal to `suggestion.label`
- have `aria-label` containing `suggestion.question`
- be keyboard focusable

- [x] **Step 4: Verify**

Run:

```bash
npm --prefix frontend run lint
```

Expected: ESLint passes for the new component.

---

## Task 5: Wire Suggestions into AI Chat

**Files:**

- Modify: `frontend/src/components/AIQuery.tsx`
- Modify: `frontend/src/components/aiQuery/AssistantMessageCard.tsx`
- Modify: `frontend/src/components/aiQuery/types.ts`

**Interfaces:**

- Consumes: backend `response.suggested_followups`.
- Consumes: frontend `buildFallbackFollowUps`.
- Produces: composer-fill interaction from suggestion click.

- [x] **Step 1: Add callback prop**

In `frontend/src/components/aiQuery/types.ts`, add to `AssistantMessageCardProps`:

```ts
onSelectFollowUp: (question: string) => void
priorQuestions: string[]
```

- [x] **Step 2: Add composer focus handler**

In `AIQuery.tsx`:

```tsx
const questionInputRef = useRef<HTMLTextAreaElement | null>(null)

const handleSelectFollowUp = useCallback((nextQuestion: string) => {
  setQuestion(nextQuestion)
  window.requestAnimationFrame(() => {
    questionInputRef.current?.focus()
  })
}, [])
```

Attach `questionInputRef` to the existing composer textarea.

- [x] **Step 3: Derive prior questions**

In `AIQuery.tsx`, derive:

```ts
const priorQuestions = useMemo(
  () => activeConversation?.messages.filter((m) => m.role === 'user').map((m) => m.content) ?? [],
  [activeConversation],
)
```

Pass `priorQuestions` and `handleSelectFollowUp` into every `AssistantMessageCard`.

- [x] **Step 4: Render suggestions in assistant card**

In `AssistantMessageCard.tsx`, compute:

```tsx
const followUps = useMemo(() => {
  if (!result) {
    return []
  }
  const backend = filterFollowUpSuggestions(result.suggested_followups ?? [], priorQuestions)
  if (backend.length > 0) {
    return backend
  }
  return buildFallbackFollowUps({ response: result, priorQuestions, t })
}, [priorQuestions, result, t])
```

Render after results and before feedback:

```tsx
<FollowUpSuggestions suggestions={followUps} onSelect={onSelectFollowUp} t={t} />
```

- [x] **Step 5: Verify behavior manually**

Run:

```bash
npm --prefix frontend run dev
```

Expected:

- A successful assistant response shows up to 3 suggestion chips.
- Clicking a chip fills the composer with the full follow-up question.
- Composer receives focus.
- The question is not auto-submitted.

---

## Task 6: Richer Result Explanation

**Files:**

- Modify: `frontend/src/components/aiQuery/resultInsight.ts`
- Modify: `frontend/src/components/aiQuery/resultInsight.test.ts`
- Modify: `frontend/src/i18n/locales/tr/core.ts`
- Modify: `frontend/src/i18n/locales/en/core.ts`

**Interfaces:**

- Consumes: `QueryResultPayload`, `TFunction`, `localeTag`.
- Produces: a concise 1-2 sentence caption.

- [x] **Step 1: Extend insight output carefully**

Keep current ranked and single-KPI cases, but return a fuller localized sentence.

Example keys:

```ts
insight_ranked_explained:
  '{{top}}, {{metric}} icin en yuksek degerle one cikiyor ({{topVal}}). Bu sonuc {{n}} {{dim}} degerinin karsilastirilmasiyla hesaplandi; aralik {{minVal}}-{{maxVal}}.'

insight_single_explained:
  '{{metric}} degeri {{val}}. Sonuc tek metrik ve tek satir olarak dondugu icin bunu KPI ozeti olarak gosteriyorum.'

insight_time_bucket_explained:
  'Bu sonuc zaman alaninin {{grain}} bazinda gruplanmasiyla hesaplandi.'
```

- [x] **Step 2: Detect time buckets conservatively**

Use column names only for generic explanatory text:

```ts
function timeGrainForColumn(name: string): 'saat' | 'gun' | 'ay' | 'yil' | null {
  const lower = name.toLocaleLowerCase('tr')
  if (lower.includes('hour') || lower.includes('saat')) return 'saat'
  if (lower.includes('day') || lower.includes('gun')) return 'gun'
  if (lower.includes('month') || lower.includes('ay')) return 'ay'
  if (lower.includes('year') || lower.includes('yil')) return 'yil'
  return null
}
```

Do not claim a grain unless the column name clearly indicates it.

- [x] **Step 3: Add tests**

Tests:

```ts
it('returns a richer ranked explanation')
it('returns a richer single KPI explanation')
it('does not invent time grain for unknown columns')
```

- [x] **Step 4: Verify**

Run:

```bash
npm --prefix frontend run test -- resultInsight
```

Expected: existing insight behavior remains covered and richer text is stable.

---

## Task 7: i18n Copy

**Files:**

- Modify: `frontend/src/i18n/locales/tr/core.ts`
- Modify: `frontend/src/i18n/locales/en/core.ts`

**Interfaces:**

- Produces localized copy consumed by Tasks 3-6.

- [x] **Step 1: Add Turkish keys**

```ts
followups_title: 'Devam etmek ister misin?',
followups_apply_aria: 'Bu takip sorusunu yaz: {{question}}',
followups_trend_label: 'Zamana gore kir',
followups_trend_question: 'Bu sonucu zamana gore daha detayli kir.',
followups_compare_label: 'En yuksekleri karsilastir',
followups_compare_question: 'Bu sonucu en yuksek degerlere gore karsilastir.',
followups_chart_label: 'Grafikte goster',
followups_chart_question: 'Bu sonucu uygun bir grafikle goster.',
```

- [x] **Step 2: Add English keys**

```ts
followups_title: 'Want to continue?',
followups_apply_aria: 'Write this follow-up question: {{question}}',
followups_trend_label: 'Break down over time',
followups_trend_question: 'Break this result down over time in more detail.',
followups_compare_label: 'Compare the top values',
followups_compare_question: 'Compare this result by the highest values.',
followups_chart_label: 'Show as a chart',
followups_chart_question: 'Show this result as a suitable chart.',
```

- [x] **Step 3: Copy review**

Check:

- Turkish labels fit small chips.
- English labels fit small chips.
- No label describes implementation details.
- Full question text is clear when inserted into the composer.

---

## Task 8: Optional Backend AI Rewrite Phase

**Files:**

- Modify: backend AI follow-up generation helper from Task 2.
- Test: matching backend tests.

**Interfaces:**

- Consumes: deterministic candidates and conversation/query context.
- Produces: validated AI-rewritten suggestions.

- [x] **Step 1: Use existing structured AI helper**

If an existing JSON-schema or structured-output helper exists in the AI package, use it. Do not create a second ad-hoc JSON extraction stack.

- [x] **Step 2: Restrict AI to rewriting/selecting**

Prompt rule:

```text
You may rewrite labels/questions and select the best candidates.
You may not introduce fields outside AVAILABLE_FIELDS.
You may not create more than 3 suggestions.
Return strict JSON only.
```

- [x] **Step 3: Validate after AI**

Always call:

```go
validated := ValidateSuggestedFollowUps(aiSuggestions, ctx.AvailableFields, ctx.PriorQuestions)
```

If `validated` is empty, return deterministic validated suggestions.

- [x] **Step 4: Test AI failure path**

Use a stub provider that returns invalid JSON or invalid fields. Expected: deterministic fallback still returns safe suggestions.

---

## Task 9: Full Verification

**Files:**

- No new files.

**Interfaces:**

- Confirms all touched frontend/backend behavior.

- [x] **Step 1: Frontend checks**

Run:

```bash
make check-frontend
```

Expected: eslint, Tailwind checks, format check, knip, tests, and build pass.

- [x] **Step 2: Backend checks if Go files changed**

Run:

```bash
gofmt -w <touched-go-files>
make lint-go
make test-go
```

Expected: lint and race tests pass.

- [x] **Step 3: gograph review if Go files changed**

Use MCP:

```text
gograph_review --uncommitted
```

Expected: blast radius matches the AI query response/follow-up generation scope.

- [x] **Step 4: Manual UX smoke**

Run:

```bash
make dev-frontend
```

With backend services available through the normal dev flow, ask a BI question that returns rows. Expected:

- Assistant answer includes richer explanation.
- Suggestions appear only for successful responses with usable result context.
- Clicking a suggestion fills and focuses the composer.
- Feedback controls still work.
- Details toggle still works.
- CSV download and chart/table controls still work.

---

## Acceptance Criteria

- Successful assistant responses can include `suggested_followups` in persisted response JSON.
- Frontend renders at most 3 follow-up chips under the assistant response.
- Follow-up chips never auto-submit in the first implementation.
- Follow-up chips fill the composer and focus it.
- Suggestions are removed when they duplicate prior user questions.
- Older responses without backend suggestions still get safe frontend fallback suggestions when result shape supports it.
- Empty/error/clarification-only responses do not show follow-up chips unless a safe suggestion exists.
- Richer explanation text stays factual and does not invent unsupported grouping or semantic meaning.
- `make check-frontend` passes.
- If backend code changes, `make lint-go`, `make test-go`, and `gograph_review --uncommitted` pass.

---

## Self-Review

- Spec coverage: The plan covers backend structured suggestions, AI validation, deterministic fallback, frontend UI, composer behavior, richer explanation copy, i18n, persistence, tests, and verification.
- Placeholder scan: No task depends on `TBD`, `TODO`, or unspecified "handle edge cases" work. Optional AI rewrite is explicitly scoped as Task 8 and can be skipped without blocking the MVP.
- Type consistency: `SuggestedFollowUp`, `SuggestedFollowUpKind`, `suggested_followups`, `filterFollowUpSuggestions`, `buildFallbackFollowUps`, and `FollowUpSuggestions` are named consistently across backend/frontend tasks.
- UI/UX fit: The first interaction behaves like an agent chat by proposing next actions without taking control away from the user.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-07-ai-chat-follow-up-suggestions.md`.

Two execution options:

1. **Subagent-Driven (recommended)** - Dispatch a fresh subagent per task, review between tasks, and keep implementation slices small.
2. **Inline Execution** - Execute tasks in this session using `superpowers:executing-plans`, with checkpoints after backend contract, frontend UI, and verification.

## Status: Implemented (verified 2026-07-07)

All 9 tasks landed across commits `cfaf2400`..`d6698918` (plus follow-up fix `ecb7ddec`). Verified against the actual codebase:

- Backend: `internal/ai/schema.go`, `internal/ai/followup_suggestions.go`, `internal/http/handlers/ai_followups.go` — wired into `finishAIRun` / `finishAIRunResult`. `go build ./...` clean, `golangci-lint run` 0 issues, `go test -race ./internal/ai/... ./internal/http/handlers/...` all pass, `gofmt` clean on touched files.
- Frontend: types, fallback helper, i18n copy, richer `resultInsight` captions all present. `npm run check` (eslint + tailwind + format + knip + vitest + tsc build) exits 0, 57/57 test files pass.
- Two file-location deviations from the plan's assumptions (implementation adapted to the real codebase structure, behavior unchanged):
  - Composer fill/focus wiring lives in `frontend/src/components/aiQuery/ChatPanel.tsx`, not `AIQuery.tsx`.
  - The chip component is `frontend/src/components/aiQuery/FollowUpSuggestionsSection.tsx`, not `FollowUpSuggestions.tsx`.

