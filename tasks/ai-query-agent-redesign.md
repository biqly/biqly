# AI Query — Agentic BI Interface Redesign (plan)

Source: user-provided 17-section product spec (2026-07-10). Goal: the AI Query
screen must read as an autonomous AI data analyst, not a generic chatbot.
Preserve the dark theme + purple accent identity. The AI never emits raw SQL;
LogicalQuery stays the contract.

Existing building blocks to reuse (do NOT rebuild): RunTrace (thinking steps),
AgentTraceCard + agent SSE stream (live steps), pendingByConversation (busy
state), conversationGroups (sidebar grouping), toolcontract step summaries,
KPI-style cards in admin (KPICard), chart tabs in AssistantMessageCard.

Status: Phases 1–5 landed (2026-07-11, gates green) except the backend-gated
items called out below. Shipped: run timeline + live card + KPI + contextual
follow-ups + chart relevance (P1/P2), composer/config/mentions/empty state
(P3), sidebar search+pin+failed (P4 §9), error/recovery banner (P5 §12), and
saved-query artifact chip (P5 §15). Deferred (need backend): right inspector
§10 (redundant w/ inline trace), plan card §11 (no plan event), approval
gates §13 (policy), memory-used §14 (response field), dashboard/schedule/alert
artifacts §15.

## Phase 1 — Run visibility (highest impact) — DONE
1. **Agent Run Timeline** (spec §1): upgrade RunTrace rows → status icon +
   duration + human explanation + expandable technical details (tool calls,
   LogicalQuery, SQL, validator output). No chain-of-thought, only observable
   actions. Backend already persists steps (agent_steps / run trace).
2. **Live run card** (spec §2): while running, render checklist card
   (done/current/pending steps, current action line, elapsed, Stop, View
   activity). Wire to existing SSE step events; today only Agent Mode streams —
   legacy pipeline shows phase events from the job (phase/phase_message).
3. **Answer hierarchy** (spec §6): direct answer → viz/KPI → key insights →
   run summary chips (duration/steps/tools/queries) → collapsible technical
   details.

## Phase 2 — Result presentation — DONE
4. **KPI card for single-value results** (spec §5): detect 1x1 result → KPI
   card instead of table; context-aware viz tab set (hide irrelevant chart
   types by result shape).
5. **Contextual follow-up chips** (spec §7): generate from result metadata
   (compare period, breakdown by dimension, top-N, explain change) — needs a
   small backend addition to include suggestion metadata in the response.

## Phase 3 — Composer & context
6. **Agent Configuration popover** (spec §9): replace the three checkboxes
   with a compact status chip + popover (context toggles, skills, execution
   caps).
7. **Composer command center** (spec §10): placeholder rotation, @ field
   / command # term + context tokens (@ and / partially exist), "Run analysis"
   primary + "Preview plan/query" secondary.
8. **Empty state** (spec §17): AI Data Analyst onboarding (sample prompts per
   datasource, capability cards).

## Phase 3 — Composer & context — DONE
(agent config popover, composer mentions @ / #, chart-relevance, empty state)

## Phase 4 — Sidebar & inspector
9. **Sidebar upgrades** (spec §11) — DONE: client-side search (title + message
   text), localStorage-backed pin with a Pinned group above the recency
   groups, per-conversation failed-run dot (derived from terminal-failed
   jobs), pin toggle in the hover action row. (Grouping + running dot were
   already done.) — `conversationPins.ts`, `SidebarConversationItem.tsx`.
10. **Right-side Agent Inspector** (spec §8) — DEFERRED. The RUN and TOOLS
    surfaces it would host are already shown inline by AgentTraceCard /
    RunTracePanel (live) and AssistantThinkingSteps (persisted); a PLAN tab
    needs the backend plan event (§11) and a CONTEXT/memory tab needs the
    `used_memories` field (§14). A standalone drawer would mostly duplicate
    the inline trace at high layout risk with little net-new data — revisit
    once the backend plan/memory fields land.

## Phase 5 — Agentic trust UX
11. **Plan card** (spec §3, "3/7 completed") — DEFERRED (needs backend). The
    SSE stream has no first-class plan event; a real N-step plan with a fixed
    total can't be synthesized honestly from the tool-call steps. Live
    progress is already conveyed by the timeline + elapsed + Stop (Phase 1).
    Land after the agent runtime emits a plan event.
12. **Error & recovery UX** (spec §12) — DONE: `runRecovery.ts` derives a
    retry/replan/recovery summary from the step list; RunTracePanel shows a
    "recovered after a failed attempt" banner (live and on reloaded answers),
    on top of the existing per-step failure reason codes.
13. **Approval gates** (spec §13) — DEFERRED (needs backend policy support:
    cost/row estimate + a pause-for-approval decision in the agent runtime).
14. **Memory UX** (spec §14) — DEFERRED (needs backend). Memory CRUD already
    exists in Settings (AIMemorySection); surfacing "which memories were used"
    / a "remember this preference" prompt needs a `used_memories` field on the
    response and server-side preference detection.
15. **Artifacts** (spec §15) — DONE (saved query): `AnswerArtifactActions`
    adds a "Saved query" chip after a successful analysis, handing the
    LogicalQuery to the Saved Questions form via its existing `?prefill=1`
    route. Add-to-dashboard/schedule/alert DEFERRED: dashboard needs a new
    prefill entry across Dashboard/DashboardList/DashboardBuilder, schedules
    require a saved skill first, and no alert feature exists yet.

Visual direction throughout (spec §16): fewer nested boxes, purple only for
primary/active/running, semantic status colors, progressive disclosure.

Each phase lands as its own dev→gate cycle. Phase 1 first.
