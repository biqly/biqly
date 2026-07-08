import type { TranslationKey } from '../../i18n'
import { cn } from '../../lib/cn'
import type { PendingAgentClarification } from '../../types/agent'
import type { RunStep } from '../../types/ai'
import {
  chatMsgAssistantClass,
  chatMsgAvatarClass,
  chatMsgClass,
  chatMsgMainClass,
} from './aiQueryClasses'
import { ClarificationCard } from './routingViz'
import { RunTracePanel } from './RunTrace'

// AgentTraceCard is the ephemeral "in-progress assistant turn" slot for
// Agent Mode (T11): it renders the live/growing step trace while a run is
// streaming, and — when the run pauses on a clarification_required event —
// the same ClarificationCard the legacy pipeline uses, wired to a resume
// call instead of a fresh question. It intentionally does NOT go through
// AssistantMessageCard/assistantMessageCardSections.tsx: that shared
// pipeline gates its own trace panel off entirely for
// `needs_clarification` messages (a reasonable default for the legacy
// ambiguity flow, which rarely has a meaningful step trace), and this task
// must not change that shared, already-tested behavior. Once the run
// reaches a terminal result/error, ChatPanel stops rendering this card and
// the real persisted assistant message (via addMessage) takes over.
export function AgentTraceCard({
  steps,
  clarification,
  onSelectClarificationChoice,
  onSkipClarification,
  t,
}: {
  steps: RunStep[]
  clarification: PendingAgentClarification | null
  onSelectClarificationChoice: (choiceId: string) => void
  onSkipClarification: () => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}) {
  if (steps.length === 0 && !clarification) {
    return null
  }
  return (
    <div className={cn(chatMsgClass, chatMsgAssistantClass)} data-testid="agent-trace-card">
      <span className={chatMsgAvatarClass} aria-hidden="true">
        ✦
      </span>
      <div className={chatMsgMainClass}>
        {steps.length > 0 ? <RunTracePanel steps={steps} defaultOpen t={t} /> : null}
        {clarification ? (
          <>
            <ClarificationCard
              question={clarification.question}
              options={clarification.choices.map((c) => c.label)}
              clarification={{
                status: 'needs_clarification',
                question: clarification.question,
                options: clarification.choices.map((c) => ({ key: c.id, label: c.label })),
              }}
              round={0}
              maxRounds={0}
              onSelect={onSelectClarificationChoice}
              onSkip={onSkipClarification}
            />
            {clarification.allowFreeText ? (
              <p className="text-foreground-faint mt-2 text-[0.78rem]">
                {t('ai_query.agent_clarification_free_text_hint')}
              </p>
            ) : null}
          </>
        ) : null}
      </div>
    </div>
  )
}
