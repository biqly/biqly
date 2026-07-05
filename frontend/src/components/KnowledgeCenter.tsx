import { useQueryParam } from '../hooks/useQueryParam'
import { useT } from '../i18n'
import { legacyLayoutClass } from '../lib/layoutClasses'
import { toggleGroupClass } from '../lib/toggleClasses'
import Glossary from './Glossary'
import { InstructionsSection } from './knowledgeCenter/InstructionsSection'
import { AIMemorySection } from './settings/AIMemorySection'
import Skills from './Skills'
import { ToggleButtonGroup } from './ui/ToggleButtonGroup'

const SECTIONS = ['instructions', 'glossary', 'saved_queries', 'memory'] as const
type KnowledgeSection = (typeof SECTIONS)[number]

function isKnowledgeSection(value: string): value is KnowledgeSection {
  return (SECTIONS as readonly string[]).includes(value)
}

/**
 * KnowledgeCenter is the single, unified governed-context surface (SP1). It
 * consolidates the formerly separate Instructions (new), Glossary, Saved
 * Queries (skills), and Memory surfaces into one datasource-scoped destination
 * with a section switcher. Each section embeds the existing, unchanged
 * component; only Instructions is net-new UI. The active section is mirrored to
 * the `?tab=` URL param so links deep-link into a section.
 */
export default function KnowledgeCenter() {
  const t = useT()
  const [tabParam, setTabParam] = useQueryParam('tab')
  const active: KnowledgeSection = isKnowledgeSection(tabParam) ? tabParam : 'instructions'

  const options = [
    { value: 'instructions', label: t('knowledge_center.tab_instructions') },
    { value: 'glossary', label: t('knowledge_center.tab_glossary') },
    { value: 'saved_queries', label: t('knowledge_center.tab_saved_queries') },
    { value: 'memory', label: t('knowledge_center.tab_memory') },
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

      {active === 'instructions' && <InstructionsSection />}
      {active === 'glossary' && <Glossary />}
      {active === 'saved_queries' && <Skills />}
      {active === 'memory' && <AIMemorySection />}
    </div>
  )
}
