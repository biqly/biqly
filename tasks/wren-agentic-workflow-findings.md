# Agentic Workflow — Live Findings & biqly Fix Plan

Date: 2026-07-04
Source: hands-on session with a reference "governed context layer" product (cloud project 16171), Agentic Mode. Question run end-to-end: *"What is the fraud rate by card brand?"* Evidence screenshots in the session scratchpad (`wren-*.png`). This complements `tasks/governed-context-layer-gap-analysis.md` — that doc is the feature catalog; this one is what the *live agent loop* actually does and where biqly falls short.

> Naming: the reference product is not advertised. Below it is "the reference agent".

---

## 1. What the reference agent actually did (observed, step by step)

1. **Composer**: single input "Ask to explore your data" + a **model & reasoning-effort selector** (`GPT-5.4 · medium`) + an **Agentic-mode toggle** + "＋" attach. Home shows **auto-generated, categorized starter questions** ("Segmentation Questions", "Comparative Questions") derived from the dataset.
2. On submit it created a **thread** and streamed a **collapsible "thinking steps" timeline** with *named* steps: `Considering measurement ambiguity` → `Querying data` → (tool) → `Running Wren query` → `Compute fraud rate by card brand` → `Search engine knowledge for SQL correction`.
3. **It called a Skill** (`analyze-data`) rather than a single text-to-SQL call. The skill **read 8 project files** before writing any SQL:
   - MDL model files: `Transactions_Cleaned.yaml`, `Cards_Cleaned.yaml`, `train_fraud_labels_1.yaml`, `relationships.yaml`
   - Knowledge **rule** files: `engine-knowledge.md`, `exclude-money-transfers.md`, `rounding.md`, `clarify-discipline.md`
4. **Clarify-before-SQL**: because "fraud rate" has an ambiguous denominator, it opened a **clarification card** — `Let's clarify some details` — with **numbered, clickable options** (1 Transaction-level / 2 Card-level / 3 Customer-level), an **"Add additional details"** free-text field, and **Skip / Send**. Identifiers rendered as **inline code chips** (`card_brand`).
5. After the answer it **drafted a reusable glossary definition** and asked a **second clarification**: *"What should I do with it? 1 Publish to project knowledge / 2 Keep as draft / 3 Discard"* — a **learning loop** that persists metric definitions back into the governed context.
6. **Answer render**: a **bar chart** (sorted desc) + a **1-2 sentence natural-language caption** ("Discover has the highest fraud rate at 0.38%, but all brands are tightly clustered between 0.32% and 0.38%"), plus a downloadable **artifact** `fraud-rate.md` (760 B) shown in a **split-pane artifact viewer** (Rendered/Source toggle, "Save to artifact").
7. **Knowledge base** (`/knowledge/file-explorer`): a **versioned markdown repo** the agent reads for context:
   - `instructions/` — rules the agent follows when generating SQL (`exclude-money-transfers.md`, `rounding.md`)
   - `sql-pairs/` — worked **question→SQL few-shot examples** (`fraud-chip-vs-darkweb.md`)
   - `README.md`, per-file **"Summary for the AI"** frontmatter for routing, **Publish** status.
   - On publish, SQL-pairs + instructions are **auto-extracted** so non-agentic ("Classic") chat modes get the same context — one source of truth.

The through-line: the agent's quality comes from a **governed, user-editable context layer** (MDL + markdown instructions + few-shot SQL-pairs + glossary), a **clarify-discipline rule**, and a **write-back learning loop** — not from a bigger prompt.

---

## 2. biqly today (to confirm during discovery, §4)

From this session's code map, biqly has: custom text-to-SQL (`internal/ai/*`), semantic models (dimensions/metrics/joins, `internal/semantic`), AI describe (`internal/ai/describe.go`), model generation (`internal/semanticgen`), a chat panel (`frontend/src/components/**/ChatPanel*`), routing/thinking viz (`routingViz.tsx`), clarifications (`aiQueryClasses.ts`), spend limits, audit channels, and a Skills endpoint (`/api/ai/skills`). What it appears to **lack** is the reference agent's *context-layer + learning-loop* spine.

---

## 3. Gaps → prioritized fixes

Ranked by leverage (P0 = biggest quality lever).

### P0-A. Governed Knowledge base (markdown instructions + few-shot SQL-pairs + glossary)
- **Gap**: biqly grounds the LLM in semantic models + column descriptions, but has no user-editable, versioned store of (a) **instructions/rules** ("exclude money transfers", "round to 2dp"), (b) **question→SQL few-shot pairs**, (c) **glossary/metric definitions** injected into the prompt.
- **Fix**: add a per-datasource/project **Knowledge base**: markdown files with frontmatter (`type: instruction|sql-pair|glossary`, `description`), a file-explorer UI (Rendered/Source), publish status, and prompt-assembly that injects the relevant instructions + top-k SQL-pairs + matching glossary terms into the text-to-SQL context. Reuse existing `/api/ai/skills` if it already models "skills".
- **Why**: this is the single largest quality/consistency lever the reference agent has.

### P0-B. Clarify-discipline (ask numbered clarifications before running SQL)
- **Gap**: biqly has a clarification surface (`aiQueryClasses.ts`), but verify it (1) triggers on **metric ambiguity** (denominator/grain), (2) blocks SQL until answered, (3) renders **numbered clickable options + free-text "add details" + Skip/Send**, and (4) is driven by an editable rule (a `clarify-discipline` instruction).
- **Fix**: an ambiguity check in the AI pipeline that, on low-confidence/ambiguous grain or metric, emits a structured clarification (options[] + allow-free-text) and pauses; the UI renders the numbered-option card (matches the visual spec already in the gap analysis §10). Inline code chips for identifiers.

### P1-C. Learning loop (write answers back into knowledge)
- **Gap**: no mechanism to capture a resolved definition (e.g. the agreed "fraud rate = …") and persist it to the knowledge base for reuse.
- **Fix**: after a query resolves an ambiguity, offer **Publish to knowledge / Keep draft / Discard**; on publish, write a `glossary` markdown entry (SQL formula + usage notes) that future prompts read. Feeds P0-A.

### P1-D. Chart + natural-language caption
- **Gap**: verify biqly auto-renders a chart for aggregate results **and** an auto-written 1-2 sentence insight caption.
- **Fix**: after result, pick a chart type from the shape (categorical→bar sorted desc, time→line) and generate a short caption ("X is highest at N%; all values cluster between A and B"). Low cost, high perceived quality.

### P1-E. Thinking-steps timeline fidelity
- **Gap**: biqly has `routingViz`; verify it shows *named* steps + tool calls (skill/query/knowledge-read) in a collapsible timeline, not just a spinner.
- **Fix**: stream named reasoning steps + tool invocations; collapsible "Show thinking steps".

### P2-F. Artifacts + split-pane viewer
- **Gap**: answers don't produce saved artifacts (chart export, glossary .md) with a viewer.
- **Fix**: represent chart/definition/exports as artifacts listed per thread, opened in a split pane (Rendered/Source), downloadable.

### P2-G. Composer: model + reasoning-effort selector; auto-suggested categorized starter questions
- **Gap**: confirm biqly's composer exposes model/effort and that home shows **data-derived, categorized** suggested questions (the gap analysis §10 already specs suggested questions — verify categorization + that they're generated from the semantic model, not static).
- **Fix**: add the selector; generate categorized starters from the model's dimensions/metrics.

### P2-H. One-source context sync to non-agentic modes
- **Gap**: if biqly has multiple query modes, ensure knowledge (instructions + SQL-pairs) feeds all of them from one store.
- **Fix**: extract instructions + SQL-pairs on publish; inject into every query path.

---

## 4. Discovery tasks (verify biqly before building)

Run these against the biqly repo to turn each "verify" above into a confirmed gap or a "already exists, needs polish". Prefer `gograph_*` for Go symbols, `Read`/`Grep` for frontend.

- [ ] **Prompt assembly**: find where the text-to-SQL prompt is built (`internal/ai/*prompt*`, gograph_query for the prompt builder). List every context source it injects (schema, descriptions, examples?). → confirms P0-A / P0-H.
- [ ] **Few-shot / examples**: search for any existing question→SQL example store or embedding retrieval of past queries. → confirms P0-A.
- [ ] **Clarifications**: read `frontend/src/**/aiQueryClasses.ts` + the backend clarification path; determine trigger conditions, whether SQL is blocked, and the render (numbered options? free-text? skip/send?). → confirms P0-B.
- [ ] **Ambiguity detection**: is there a confidence/ambiguity gate in the AI pipeline (`internal/ai`)? → confirms P0-B.
- [ ] **Charts + captions**: inspect the chat result renderer (`ChatPanel`/result components); does it auto-pick a chart and write a caption? → confirms P1-D.
- [ ] **Thinking steps**: read `routingViz.tsx`; are steps named + tool calls shown + collapsible? → confirms P1-E.
- [ ] **Skills endpoint**: what does `/api/ai/skills` model today (the handler + `metadata.SkillRow`)? Is it a knowledge/skills store we can extend for P0-A? → confirms P0-A scope.
- [ ] **Suggested questions**: find the home/suggested-questions generator; are they data-derived + categorized? → confirms P2-G.
- [ ] **Artifacts**: does the chat persist any artifact objects per thread? → confirms P2-F.

---

## 5. Recommended sequencing

1. **Discovery (§4)** — a half-day pass; convert each verify into confirmed/partial/exists.
2. **P0-A Knowledge base** (spec + plan of its own) — the spine everything else feeds.
3. **P0-B Clarify-discipline** — pairs with the §10 visual spec already approved.
4. **P1-C Learning loop** → writes into P0-A.
5. **P1-D caption + P1-E thinking-steps** — cheap perceived-quality wins, can run in parallel.
6. **P2** items last.

Each P-item that survives discovery should get its own brainstorm → spec → plan (like the semantic-canvas UX work), not be lumped into one mega-plan.

---

## 6. Evidence

Screenshots (session scratchpad): `wren-home.png` (starter questions), `wren-q1-final.png` (numbered clarification card), `wren-q1-chart.png` (Publish-to-knowledge learning loop), `wren-q1-done.png` (chart + caption + artifact split-pane), `wren-knowledge3.png` (Knowledge file-explorer: instructions/ + sql-pairs/).
