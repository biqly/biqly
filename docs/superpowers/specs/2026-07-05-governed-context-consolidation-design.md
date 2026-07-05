# Governed Context Layer Consolidation — Design

Date: 2026-07-05
Status: Approved (design). Implement incrementally, one sub-project per spec→plan→build cycle.
Scope: biqly AI grounding surfaces (Knowledge / Glossary / Few-shot / Skills / Prompt Templates) + the AI-query answer UX.

## Problem

biqly has FIVE separate grounding surfaces, all injected into the text-to-SQL prompt, presented as five confusing pages:
- `business_glossary_terms` (Glossary page + GlossaryEnrichPanel) — terms + AIContext (unit/null-meaning/business_rules).
- `ai_confirmed_queries` (FewShotExamples page) — Q→LogicalQuery few-shot pairs, embedding-RAG recall.
- `ai_skills` (Skills page) — executable parameterized LogicalQuery templates.
- `memory_entries` — remembered facts.
- `ai_prompt_templates` (Prompt Templates page) — per-locale system prompt sections.

Users can't tell Skills from Examples from Knowledge. There is no place for free-form business rules (wren's `instructions/`). The AI answer is not presented like a real AI app.

## Locked decisions

- **Consolidation model = unified surface, keep structured** (not pure-markdown, not two-source hybrid). One **Knowledge Center** nav over the existing structured DB stores + embedding RAG; only the UI/mental-model consolidates. Adds ONE new free-form store for `instructions`.
- **Skills + Examples = one "Saved Query" concept.** Merge `ai_skills` + `ai_confirmed_queries` into a single record type: Question + LogicalQuery + optional parameters. Each record serves BOTH roles: (a) embedding-RAG few-shot grounding, (b) directly runnable skill. One-time DB migration.
- **"/" selection = grounding-first.** In the query composer, `/` opens a Saved Query picker; picked items are injected as strong grounding for the current question. Direct execution is a separate "Run" action in the library/detail (param form when needed).
- **Glossary stays structured** (valuable, distinct grounding type; injected). Not converted to markdown.
- **AI answer = streamed, left-aligned, like a real AI app** (new, SP6).

## Sub-projects (build order)

### SP1 — Unified Knowledge Center + Saved Query merge (foundation, biggest)
One `Knowledge Center` page (replaces the separate Knowledge/Glossary/FewShot/Skills pages), datasource-scoped, with sections:
- **Instructions** *(new store)*: free-form markdown business rules. New table `ai_instructions` (id, datasource_id, title, body_md, is_active, updated_at). Injected into the prompt as a `## Business Rules` block (extend `buildPromptTemplateData` / a new `writeInstructions` in `internal/ai/prompt/`).
- **Glossary**: unchanged data (`business_glossary_terms`), moved under this nav.
- **Saved Queries** *(merged)*: new table `ai_saved_queries` = union of `ai_skills` + `ai_confirmed_queries` (Question, LogicalQuery JSON, params JSON nullable, question_embedding, datasource_id, model_id, is_active, tags, source). Migration copies both tables in; recall (`RecallFewShot`) and execution (`AISkillsHandler.Run`) both point at it. Keep the embedding-RAG path intact.
- **Memory**: unchanged (`memory_entries`), moved under this nav.
Prompt injection continues to assemble from the same substrates (now: instructions + glossary + saved-queries few-shot + memories).

### SP2 — AI-assisted Saved Query authoring
"Prepare a saved query for: {NL}" → reuse the text-to-SQL generation (NL→LogicalQuery) to draft a Saved Query (LogicalQuery + suggested `{{param}}` placeholders + name/description); user reviews + saves. Mirrors the metadata auto-describe UX. New endpoint `POST /api/ai/saved-queries/draft`.

### SP3 — Query-time selection
- **Auto-find toggle** in the composer: when on, embedding-RAG auto-injects the top matching Saved Queries as grounding (this is today's few-shot recall, made a visible toggle). Off → only "/"-selected ones.
- **"/" manual multi-select**: typing `/` in the composer opens a Saved Query picker (mention-style, like the existing `@` field mentions); selected items are passed to the backend as an explicit grounding set for the current question.
- **Run**: a separate action (library/detail) runs a Saved Query directly through the governed path (param form when params exist).

### SP4 — Prompt template improvements (independent, low risk)
- Strengthen `system_rules`/`output_format` `formula`/`percent_change` period-over-period examples (the "vs previous week" case).
- Add `'formula'` to the frontend `SelectField` union (`frontend/src/types/ai.ts`) — the backend already produces it but it is untyped/unrendered on the client.
- Reflect the empty-response "OUTPUT ONLY JSON" emphasis in the base template.

### SP5 — Deprecation / cleanup
- Remove the standalone `FewShotExamples`, `Skills`, `Glossary`, old `Knowledge` pages/routes → sections under the one Knowledge Center. Simplify nav.
- Keep all DATA (glossary, memories) — only pages consolidate. Retire truly-unused helpers (e.g. `GlossaryEnrichPanel` if redundant after the merge) only if a focused review proves them obsolete.
- Drop the old `ai_skills` / `ai_confirmed_queries` tables only AFTER the SP1 migration is verified in prod.

### SP6 — AI answer UX: streamed, left-aligned (do first — freshly requested, self-contained)
The NL answer (shipped in rev 52 as `AIResult.Answer`) must present like a real AI app:
- **Left-aligned assistant area** next to the ✦ avatar (NOT inside the result container). The prose answer renders there; the result chart/table renders BELOW it when it arrives.
- **Typewriter streaming**: render the answer progressively (token/char-by-char) on arrival, respecting `prefers-reduced-motion` (instant when reduced). Implementation: frontend typewriter over the already-returned `answer` string — no backend streaming needed for a 1-2 sentence answer (answer synthesis is server-side post-query, so even real token streaming could only start after the query completes; a typewriter on arrival is visually equivalent and far lower risk). Real SSE streaming is a possible later upgrade if longer answers warrant it.
- Layout: restructure the assistant message in `frontend/src/components/aiQuery/AssistantMessageCard.tsx` / `assistantMessageCardSections.tsx` so: (row) ✦ avatar + assistant name/time; (below, left) streamed answer text; (below) the existing result card (chart/table/details). Keep the deterministic caption fallback when `answer` is empty.

## Cross-cutting

- Migrations additive + reversible where possible; the table-drop (SP5) is gated on prod verification.
- Every step: `make lint-go` + `make test-go` + `make check-frontend` green; `gograph_review` after Go edits; `TestConfigDocSync` for any new env var.
- i18n (EN+TR) for all new UI text.

## Out of scope (for now)
- Pure-markdown migration of glossary/memories.
- Real SSE token streaming of the answer (typewriter first).
- Cross-workspace knowledge sharing / versioning beyond what exists.
