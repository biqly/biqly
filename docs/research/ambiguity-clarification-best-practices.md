# Ambiguity & Clarification in Text2SQL: Best Practices

> Comparative analysis: Biqly vs common industry patterns, with concrete improvement recommendations.

## 1. Problem Domain

Users ask questions in natural language. Those questions are frequently underspecified:

| Ambiguity Type | Example | Root Cause |
|---|---|---|
| **Synonym collision** | "Show revenue" | `revenue` matches both `total_revenue` and `net_revenue` |
| **Homonym column** | "Sales by product" | `product` could be `product_name` or `product_category` |
| **Implicit filter** | "Last quarter" | Which fiscal calendar? Calendar vs fiscal alignment? |
| **Aggregation ambiguity** | "Average order" | Average of `order_total` or count of `order_items`? |
| **Table routing** | "Customer count" | `customers` table or `users` with `role=customer`? |

A robust Text2SQL system must **detect** these cases and **resolve** them without frustrating the user.

---

## 2. Biqly's Current Architecture

### 2.1 Flow Diagram

```
User Question
     │
     ▼
 parseAndRouteAIQuery (sync) / executeAIQueryPhase (async job)
     │
     ├── Table Routing → NeedsClarification? → Return clarification options
     │
     ├── ClarificationChoice present?
     │   └── resolveClarificationChoice() → Rewrites question, sets clarificationResolved=true
     │
     └── standardProcessOptions()
          └── If CheckEnabled && !clarificationResolved → Add WithAmbiguityCheck(true)
               └── ProcessQuestion() → LLM ambiguity pass (pre-LLM deterministic + optional LLM-backed)
                    └── Returns clarification response OR proceeds to SQL generation
```

### 2.2 Key Components

| Component | File | Role |
|---|---|---|
| `AmbiguityConfig` | `internal/config/config.go` | Feature flags + thresholds (CheckEnabled, ConfidenceThreshold, MaxOptions, LLMEnabled) |
| `resolveClarificationChoice()` | `internal/http/handlers/ai.go:163` | Free function: resolves user choice → rewrites question |
| `h.resolveClarificationChoice()` | `internal/http/handlers/ai.go:176` | Method: wraps free function + sets `clarificationResolved` flag + metrics |
| `standardProcessOptions()` | `internal/http/handlers/ai.go:229` | Builds options slice; includes ambiguity check only when flag is unset |
| `WithAmbiguityCheck()` | `internal/ai/service.go:227` | ProcessOption that enables deterministic semantic clarification |
| `executeAIQueryPhase()` | `internal/http/handlers/ai_job_exec.go:48` | Async job path; calls method via `&req` pointer |
| Prometheus metrics | `internal/platform/observability/metrics.go` | `biqly_ambiguity_detected_total`, `biqly_ambiguity_clarified_total`, latency histogram |

### 2.3 What Went Wrong (Bug 2)

The async job path (`ai_job_exec.go`) was calling the **free function** `resolveClarificationChoice()` instead of the **method** `h.resolveClarificationChoice()`. The free function rewrites the question but never sets `clarificationResolved=true`. On the next iteration, the guard at `ai.go:238` still evaluated to true, injecting `WithAmbiguityCheck(true)` again, creating an infinite clarification loop.

**Fix**: Move the flag-setting logic into the shared method; ensure both sync and async paths call the method via pointer receiver.

### 2.4 Strengths

1. **Two-tier ambiguity detection**: Deterministic (synonym/homonym rules) + LLM-backed fallback
2. **Configurable thresholds**: Confidence, max options, per-environment toggle
3. **Structured metrics**: Detection, clarification, and latency histograms by source
4. **Glossary integration**: Business glossary feeds into ambiguity analysis
5. **Async job support**: Clarification works through NATS-based job queue

---

## 3. Agent-Orchestrated Text2SQL (Industry Alternative)

### 3.1 Philosophical Difference

A common alternative architecture treats **ambiguity detection as agent-orchestrated capability**, not a single built-in engine flag.

> Correctness is often modeled as composable primitives (schema linking, profiling, trace, retry) that an agent invokes, rather than one opaque pipeline step.

Typical correctness pillars in agent-first systems:

| Pillar | Where it Lives |
|---|---|
| **Schema linking** | Semantic model + vector memory retrieval |
| **Value profiling** | Connector behavior, profiling workflows, instructions |
| **Ambiguity detection** | **Agent skill / orchestration** |
| **Generation trace** | Dry-plan / explain-plan tooling |
| **Retry and repair** | Structured errors, dry-run, agent retry |
| **Eval** | Golden NL-SQL eval workflows |

### 3.2 How Agent-First Systems Handle Ambiguity

These systems often **do not** expose a single `ambiguity_check` flag in the query engine. Instead:

1. **Semantic modeling layer**: Business context lives in declarative model files — entities, views, relationships, instructions, and structured `ai_context` (synonyms, descriptions, business rules).

2. **Vector memory store**: Schema items and past NL-SQL pairs are embedded and indexed. Retrieval fetches relevant context; recall finds similar past queries.

3. **Enrich-context workflow**: A skill or admin tool fills context gaps — enum labels, units, NULL semantics, synonyms, soft-delete filters, etc. May run interactively or in batch.

4. **Agent-driven clarification**: The LLM agent decides when and how to clarify; deterministic pre-checks are optional or minimal.

5. **Query classification**: Exploratory vs. analytical intent can change how much context is fetched.

### 3.3 Typical Agent-First Pipeline

```
User Question (via agent)
     │
     ▼
Agent decides: needs context? → memory fetch + recall
     │
     ▼
Agent writes SQL using semantic model names
     │
     ▼
Engine plans (semantic layer → CTE → transpile → execute)
     │
     ▼
Results returned to agent → Agent interprets for user
     │
     ▼
Store confirmed NL-SQL pair (learn from success)
```

---

## 4. Comparative Analysis

| Dimension | Biqly (backend-orchestrated) | Agent-first (industry pattern) |
|---|---|---|
| **Ambiguity detection** | Built-in, deterministic + LLM | Agent-driven, optional pre-checks |
| **Resolution mechanism** | User picks from options → question rewrite | Agent asks free-form clarification |
| **Context storage** | Glossary tables in DB | Declarative semantic model + vector memory |
| **Learning loop** | Implicit (few-shot from glossary) | Explicit (store confirmed NL-SQL pairs) |
| **Scope of context** | Per-datasource glossary | Per-project model + instructions + memory |
| **Observability** | Prometheus metrics (detection, clarification, latency) | Often agent-side logging only |
| **Sync vs Async** | Both paths supported | Often sync CLI/SDK |
| **Business semantics** | Glossary + synonym detector | Semantic layer + ai_context + enrich workflow |
| **Type safety** | Go, compile-time checks | Varies by stack |
| **Control model** | Backend-driven flow | Agent-driven flow |

### Where Biqly is Stronger

1. **Deterministic ambiguity detection**: Catches synonym/homonym collisions before the LLM sees them. Agent-first stacks often delegate detection entirely to the model.
2. **Dual-path processing**: Sync HTTP + async job queue with proper state management.
3. **Structured metrics**: Full observability on detection, clarification, and latency percentiles.
4. **Configurable guardrails**: Confidence thresholds, max options, per-environment feature flags.

### Where Agent-First Patterns Are Stronger

1. **Context richness**: A full semantic layer can carry more business semantics than a flat glossary — relationships, calculated fields, views, instructions, structured `ai_context`.
2. **Learning loop**: A confirmed-query memory store grows a corpus of NL-SQL pairs that improve future queries. Biqly is still building this (P3).
3. **Agent flexibility**: The agent decides when and how to clarify without a fixed server-side flow.
4. **Context enrichment tooling**: Systematic gap-filling (enums, units, NULL semantics, synonyms) beyond manual glossary edits.

---

## 5. Best Practices for Biqly

Based on the comparative analysis and industry patterns (AWS, academic surveys, Google Cloud):

### 5.1 Immediate (Fix Architecture Gaps)

#### P0: Eliminate Sync/Async Divergence

**Current risk**: The Bug 2 class of bugs — sync and async paths diverge in behavior.

**Recommendation**: Introduce a single `ProcessContext` struct that both paths construct identically:

```go
type ProcessContext struct {
    Question              string
    ClarificationChoice   string
    ClarificationResolved bool
    DatasourceID          string
    // ... other fields
}

func (h *AIHandler) buildProcessContext(req aiQueryRequest) *ProcessContext {
    return &ProcessContext{
        Question:            req.Question,
        ClarificationChoice: req.ClarificationChoice,
        DatasourceID:        req.DatasourceID,
    }
}

func (pc *ProcessContext) Resolve(ctx context.Context, ...) error {
    // Single resolution path — no sync/async split
}
```

Both `parseAndRouteAIQuery` and `executeAIQueryPhase` should call `buildProcessContext` + `Resolve`. The guard in `standardProcessOptions` reads `pc.ClarificationResolved`. This makes it structurally impossible for one path to miss the flag.

#### P1: Add a Clarification Round Counter

Infinite loops happen when a clarification resolves but the system still detects ambiguity on the rewritten question. Add a hard cap:

```go
const maxClarificationRounds = 2

type ProcessContext struct {
    // ...
    clarificationRound int
}

func (pc *ProcessContext) ShouldCheckAmbiguity() bool {
    return pc.clarificationRound < maxClarificationRounds && !pc.ClarificationResolved
}
```

### 5.2 Short-Term (Context Richness)

#### P2: Enrich Glossary with Structured ai_context

Current glossary entries are flat key-value pairs. Add structured context:

```sql
ALTER TABLE glossary_entries ADD COLUMN ai_context JSONB;
-- { "synonyms": ["revenue", "gelir", "ciro"],
--   "unit": "TRY",
--   "null_meaning": "not yet invoiced",
--   "business_rules": ["exclude cancelled orders"] }
```

Feed this into both ambiguity detection and the LLM prompt.

#### P3: NL-SQL Memory Store (Learn from Success)

When a user accepts a generated query, store the NL-SQL pair:

```
User question: "Ciro göster"
Generated SQL: SELECT SUM(total) FROM orders WHERE status != 'cancelled'
User accepted: ✅
→ Store: {nl: "Ciro göster", sql: "SELECT SUM(total)...", tags: ["user-confirmed"]}
```

On future queries, semantic recall over the memory store retrieves similar past queries as few-shot examples. This creates a **positive feedback loop** that reduces ambiguity over time.

#### P4: Structured Enrich-Context Workflow

Create a first-class "enrich context" endpoint (or CLI command) that:

1. Reads the semantic model + glossary + sample data
2. Detects gaps: columns without descriptions, enum values without labels, synonym collisions
3. Produces a gap report with suggested enrichments
4. User confirms → writes back to glossary/instructions

This mirrors the industry enrich-context pattern, adapted to Biqly's backend-orchestrated architecture.

### 5.3 Medium-Term (Architecture Evolution)

#### P5: Multi-Tier Clarification Strategy

Replace the single `WithAmbiguityCheck(bool)` with a tiered approach:

| Tier | When | How | Cost |
|---|---|---|---|
| **Tier 0: Routing** | Table/column ambiguity from routing | Deterministic, return options | Free |
| **Tier 1: Synonym** | Synonym/homonym collision | Deterministic glossary lookup | Free |
| **Tier 2: Semantic** | Low-confidence interpretation | LLM-backed analysis | ~$0.01 |
| **Tier 3: Interactive** | User picks wrong option twice | Agent-driven multi-turn clarification | ~$0.05 |

Each tier escalates only if the previous tier couldn't resolve. The current code has Tier 0 + Tier 2 conflated.

#### P6: Generation Trace (Dry-Plan Pattern)

Add a `trace` field to AI responses showing:

```
1. Routed to: orders (confidence: 0.92)
2. Columns resolved: total → order_total, date → order_date
3. Ambiguity check: PASSED (no synonyms matched)
4. SQL generated: SELECT SUM(order_total) FROM orders...
5. Expanded SQL (with CTEs): ...
```

This helps users understand *why* an ambiguity was detected and *what* the system understood.

#### P7: Eval Regression Suite for Ambiguity

Extend `make eval-regression` with golden cases specifically for ambiguity:

```yaml
- question: "Satışları göster"
  expected: clarification (synonym: satis_total vs satis_count)
  dialect: postgres
- question: "Show revenue for Q1"
  clarification_choice: "ambiguity:0:1"
  expected_sql: "SELECT SUM(net_revenue) FROM orders WHERE ..."
```

---

## 6. Summary: Key Takeaways

1. **Single resolution path**: Sync and async must go through identical code. Extract a `ProcessContext` to make divergence structurally impossible.
2. **Hard cap on clarification rounds**: Prevent infinite loops by design, not by flag.
3. **Rich context > clever detection**: Agent-first systems succeed when the semantic layer carries enough business meaning that ambiguity is rare. Invest in context enrichment, not only detection sophistication.
4. **Learn from success**: Store confirmed NL-SQL pairs. Use them as few-shot examples and for semantic search. This is the single highest-ROI improvement.
5. **Tiered escalation**: Not every ambiguity needs an LLM call. Route → Synonym → Semantic → Interactive, escalating cost and latency only when needed.
6. **Generation trace**: Show users what the system understood. Reduces frustration and builds trust.
7. **Eval coverage**: Golden ambiguity cases prevent regressions like Bug 2 from reaching production.

---

## Sources

- AWS — "Enterprise-grade NL2SQL using LLMs": https://aws.amazon.com/blogs/machine-learning/enterprise-grade-natural-language-to-sql-generation-using-llms/
- Google Cloud — "Techniques for improving text-to-SQL": https://cloud.google.com/blog/products/databases/techniques-for-improving-text-to-sql
- Biqly codebase: `internal/http/handlers/ai.go`, `internal/http/handlers/ai_job_exec.go`, `internal/ai/service.go`, `internal/config/config.go`
