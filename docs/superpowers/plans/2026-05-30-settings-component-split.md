# Settings Component Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split Settings security UI into focused components while preserving
all existing behavior.

**Architecture:** Keep API calls, state, errors, and modal orchestration in
`Settings.tsx`. Extract controlled presentational components and cover the
shared OTP normalization rule with Vitest.

**Tech Stack:** React 19, TypeScript 5.7, Vitest, Vite 6

---

## Task 1: Extract OTP behavior

- [x] Add a failing unit test for numeric-only, six-character OTP
  normalization.
- [x] Add `settings/otp.ts` and make the focused test pass.
- [x] Add the controlled `settings/OTPCodeInput.tsx` component.

## Task 2: Extract recovery-code presentation

- [x] Add `settings/RecoveryCodesDisplay.tsx`.
- [x] Preserve clipboard copy and existing translated alerts.

## Task 3: Extract passkey presentation

- [x] Add `settings/PasskeyTable.tsx`.
- [x] Move loading, empty, and table rendering from the parent.

## Task 4: Extract MFA presentation

- [x] Add `settings/MFASection.tsx`.
- [x] Move status and regenerated recovery-code rendering from the parent.

## Task 5: Integrate and verify

- [x] Wire the extracted components into `Settings.tsx`.
- [ ] Mark the Settings Phase 3 TODO item complete with the resulting parent
  line count.
- [x] Run focused tests, the full frontend test suite, the production build,
  and `git diff --check`.
