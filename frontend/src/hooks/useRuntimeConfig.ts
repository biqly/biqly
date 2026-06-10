import { useCallback, useEffect, useState } from 'react'

import {
  type AIAdminRuntimeConfig,
  type AIAdminRuntimeConfigUpdate,
  getAIAdminConfig,
  updateAIAdminConfig,
} from '../api/aiAdmin'

/** Editable copy of the admin-tunable runtime config knobs. */
export interface RuntimeConfigDraft {
  ambiguity: {
    check_enabled: boolean
    confidence_threshold: number
    max_options: number
    tiered_enabled: boolean
    max_llm_tier_per_question: number
  }
  pii: {
    detection_threshold: number
  }
  memory: {
    recall_enabled: boolean
    recall_limit: number
  }
}

export function draftFromConfig(config: AIAdminRuntimeConfig): RuntimeConfigDraft {
  return {
    ambiguity: {
      check_enabled: config.ambiguity.check_enabled,
      confidence_threshold: config.ambiguity.confidence_threshold,
      max_options: config.ambiguity.max_options,
      tiered_enabled: config.ambiguity.tiered_enabled,
      max_llm_tier_per_question: config.ambiguity.max_llm_tier_per_question,
    },
    pii: { detection_threshold: config.pii.detection_threshold },
    memory: {
      recall_enabled: config.memory.recall_enabled,
      recall_limit: config.memory.recall_limit,
    },
  }
}

/** Clamps to [min, max]; NaN falls back to min. Mirrors backend validation. */
export function clampNumber(value: number, min: number, max: number): number {
  if (Number.isNaN(value)) {
    return min
  }
  return Math.max(min, Math.min(max, value))
}

/**
 * Builds the PUT body from a draft, clamping every numeric knob into the
 * backend's accepted ranges so a fat-fingered input never round-trips a 400.
 */
export function buildRuntimeConfigUpdate(draft: RuntimeConfigDraft): AIAdminRuntimeConfigUpdate {
  return {
    ambiguity: {
      check_enabled: draft.ambiguity.check_enabled,
      confidence_threshold: clampNumber(draft.ambiguity.confidence_threshold, 0, 1),
      max_options: clampNumber(draft.ambiguity.max_options, 1, 10),
      tiered_enabled: draft.ambiguity.tiered_enabled,
      max_llm_tier_per_question: clampNumber(draft.ambiguity.max_llm_tier_per_question, 0, 10),
    },
    pii: {
      detection_threshold: clampNumber(draft.pii.detection_threshold, 0.05, 1),
    },
    memory: {
      recall_enabled: draft.memory.recall_enabled,
      recall_limit: clampNumber(draft.memory.recall_limit, 1, 10),
    },
  }
}

/**
 * Loads and saves the AI runtime config (/api/ai/admin/config). `save` throws
 * on failure so callers can surface the backend's field-specific message.
 */
export function useRuntimeConfig() {
  const [config, setConfig] = useState<AIAdminRuntimeConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const reload = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setConfig(await getAIAdminConfig())
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void reload()
  }, [reload])

  const save = useCallback(async (update: AIAdminRuntimeConfigUpdate) => {
    setSaving(true)
    try {
      const data = await updateAIAdminConfig(update)
      setConfig(data)
      return data
    } finally {
      setSaving(false)
    }
  }, [])

  return { config, loading, error, saving, save, reload }
}
