import { useState } from 'react'

import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import {
  feedbackBtnClass,
  feedbackCatBtnClass,
  feedbackCategoriesClass,
  feedbackFormClass,
  feedbackLearnedBadgeClass,
  feedbackRowClass,
} from './aiQueryClasses'
import type { FeedbackCatKey } from './types'
import { FEEDBACK_CAT_KEYS } from './types'

interface FeedbackSectionProps {
  // Resolves to true when the backend stored the pair in the memory store
  // ("learned"), so the section can surface that to the user.
  onSubmitPositive: () => Promise<boolean>
  onSubmitNegative: (categories: FeedbackCatKey[], text: string) => void
}

export function FeedbackSection({ onSubmitPositive, onSubmitNegative }: FeedbackSectionProps) {
  const t = useT()
  const [userFeedback, setUserFeedback] = useState<'positive' | 'negative' | null>(null)
  const [learned, setLearned] = useState(false)
  const [showFeedbackForm, setShowFeedbackForm] = useState(false)
  const [feedbackCategories, setFeedbackCategories] = useState<FeedbackCatKey[]>([])
  const [feedbackText, setFeedbackText] = useState('')

  const submitFeedback = (rating: 'positive' | 'negative') => {
    setUserFeedback(rating)
    if (rating === 'positive') {
      void onSubmitPositive().then(setLearned)
    } else {
      setShowFeedbackForm(true)
    }
  }

  const submitNegative = () => {
    onSubmitNegative(feedbackCategories, feedbackText)
    setShowFeedbackForm(false)
    setFeedbackCategories([])
    setFeedbackText('')
  }

  return (
    <>
      <div className={feedbackRowClass}>
        <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginRight: '0.5rem' }}>
          {t('ai_query.feedback_helpful')}
        </span>
        <button
          type="button"
          className={feedbackBtnClass(userFeedback === 'positive', false)}
          aria-label={t('ai_query.feedback_positive_aria')}
          onClick={() => submitFeedback('positive')}
        >
          👍
        </button>
        <button
          type="button"
          className={feedbackBtnClass(userFeedback === 'negative', true)}
          aria-label={t('ai_query.feedback_negative_aria')}
          onClick={() => submitFeedback('negative')}
        >
          👎
        </button>
        {learned && (
          <span className={feedbackLearnedBadgeClass} role="status">
            ✓ {t('ai_query.feedback_learned')}
          </span>
        )}
      </div>
      {showFeedbackForm && (
        <div className={feedbackFormClass}>
          <p style={{ fontSize: '0.8rem', marginBottom: '0.4rem', color: 'var(--text-secondary)' }}>
            {t('ai_query.feedback_what_wrong')}
          </p>
          <div className={feedbackCategoriesClass}>
            {FEEDBACK_CAT_KEYS.map((cat) => (
              <button
                type="button"
                key={cat}
                className={feedbackCatBtnClass(feedbackCategories.includes(cat))}
                onClick={() =>
                  setFeedbackCategories((prev) =>
                    prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat],
                  )
                }
              >
                {t(cat)}
              </button>
            ))}
          </div>
          <textarea
            value={feedbackText}
            onChange={(e) => setFeedbackText(e.target.value)}
            placeholder={t('ai_query.feedback_placeholder')}
            rows={2}
            style={{ width: '100%', fontSize: '0.8rem', resize: 'vertical' }}
          />
          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.4rem' }}>
            <button
              type="button"
              className={buttonClass('primary', { size: 'sm' })}
              onClick={submitNegative}
            >
              {t('ai_query.feedback_submit')}
            </button>
            <button
              type="button"
              className={buttonClass('ghost', { size: 'sm' })}
              onClick={() => {
                setShowFeedbackForm(false)
                setFeedbackCategories([])
                setFeedbackText('')
              }}
            >
              {t('ai_query.feedback_cancel')}
            </button>
          </div>
        </div>
      )}
    </>
  )
}
