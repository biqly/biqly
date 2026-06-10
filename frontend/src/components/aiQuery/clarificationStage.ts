// Mirrors backend `maxClarificationRounds` (internal/http/handlers/ai_context.go):
// at this round the server runs the Tier-3 interactive pass; past it, ambiguity
// checks are bypassed and the server answers with its best guess.
export const MAX_CLARIFICATION_ROUNDS = 2

export interface ClarificationStage {
  /** 1-based round carried on the wire; 0 when the response has none. */
  round: number
  /** Display round, capped so "3/2" never renders. */
  displayRound: number
  interactiveTier: boolean
  capReached: boolean
}

// deriveClarificationStage classifies a response's clarification_round into
// the UI stage: normal rounds, the Tier-3 interactive pass, or past-cap.
export function deriveClarificationStage(round: number | undefined): ClarificationStage {
  const r = round ?? 0
  return {
    round: r,
    displayRound: Math.min(r, MAX_CLARIFICATION_ROUNDS),
    interactiveTier: r === MAX_CLARIFICATION_ROUNDS,
    capReached: r > MAX_CLARIFICATION_ROUNDS,
  }
}
