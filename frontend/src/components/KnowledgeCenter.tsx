import { useQueryParam } from '../hooks/useQueryParam'
import { useT } from '../i18n'
import { legacyLayoutClass } from '../lib/layoutClasses'
import { toggleGroupClass } from '../lib/toggleClasses'
import { KnowledgeBasePage } from './knowledge/KnowledgeBasePage'
import Skills from './Skills'
import { ToggleButtonGroup } from './ui/ToggleButtonGroup'

const SECTIONS = ['files', 'saved_queries'] as const
type KnowledgeSection = (typeof SECTIONS)[number]

function isKnowledgeSection(value: string): value is KnowledgeSection {
  return (SECTIONS as readonly string[]).includes(value)
}

/**
 * KnowledgeCenter hosts the markdown Knowledge base (WrenAI-style file tree of
 * glossary/instructions/metrics/sql-pairs documents — the primary surface) and
 * the runnable Skills library (parameterized LogicalQuery templates, which
 * markdown cannot express). The former Instructions/Glossary form tabs were
 * replaced by the knowledge files themselves: publishing a file extracts the
 * equivalent structured records. Memory lives in Settings → Configuration.
 * The active section mirrors to `?tab=` so existing deep links
 * (`?tab=saved_queries`) keep working.
 */
export default function KnowledgeCenter() {
  const t = useT()
  const [tabParam, setTabParam] = useQueryParam('tab')
  const active: KnowledgeSection = isKnowledgeSection(tabParam) ? tabParam : 'files'

  const options = [
    { value: 'files', label: t('knowledge_base.kb_tab_files') },
    { value: 'saved_queries', label: t('knowledge_base.kb_tab_skills') },
  ] as const

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <ToggleButtonGroup
        ariaLabel={t('knowledge_center.nav_aria')}
        className={toggleGroupClass('flex-wrap')}
        toggleButtons
        value={active}
        onChange={(next: KnowledgeSection) => setTabParam(next)}
        options={options}
      />

      {active === 'files' && <KnowledgeBasePage />}
      {active === 'saved_queries' && <Skills />}
    </div>
  )
}
