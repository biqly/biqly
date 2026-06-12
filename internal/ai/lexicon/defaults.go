package lexicon

// DefaultEntries returns embedded lexicon rows used to seed ai_nl_lexicon and as
// fallback when the DB is unreachable or a domain is empty (ADR-0001 K5). The
// per-locale split is for management; matching uses the union, which must stay
// equal to the previously hardcoded mixed-locale lists (behavior-preserving).
// token_synonym and metric_synonym ship no defaults here: the routing lexicon
// JSON remains their source until DİL-2.
func DefaultEntries() []Entry {
	return []Entry{
		// --- temporal_phrase (from internal/ai/ambiguity/temporal_detector.go) ---
		{Locale: "tr", Domain: DomainTemporalPhrase, Key: "geçen ay", InterpretationKeys: []string{"prev_calendar_month", "rolling_30d"}},
		{Locale: "tr", Domain: DomainTemporalPhrase, Key: "son zamanlarda", InterpretationKeys: []string{"last_week", "last_month", "last_quarter"}},
		{Locale: "tr", Domain: DomainTemporalPhrase, Key: "yakın zamanda", InterpretationKeys: []string{"last_week", "last_month"}},
		{Locale: "tr", Domain: DomainTemporalPhrase, Key: "geçen hafta", InterpretationKeys: []string{"prev_calendar_week", "rolling_7d"}},
		{Locale: "tr", Domain: DomainTemporalPhrase, Key: "bu yıl", InterpretationKeys: []string{"ytd", "last_12m"}},
		{Locale: "en", Domain: DomainTemporalPhrase, Key: "last month", InterpretationKeys: []string{"prev_calendar_month", "rolling_30d"}},
		{Locale: "en", Domain: DomainTemporalPhrase, Key: "recently", InterpretationKeys: []string{"last_week", "last_month"}},
		{Locale: "en", Domain: DomainTemporalPhrase, Key: "lately", InterpretationKeys: []string{"last_week", "last_month"}},
		{Locale: "en", Domain: DomainTemporalPhrase, Key: "last week", InterpretationKeys: []string{"prev_calendar_week", "rolling_7d"}},

		// --- grain_synonym: union of routing/time_grains.go and semanticgen/generator.go lists ---
		{Locale: "en", Domain: DomainGrainSynonym, Key: "year", Terms: []string{"year", "years", "yearly", "annual", "per year", "by year"}},
		{Locale: "tr", Domain: DomainGrainSynonym, Key: "year", Terms: []string{"yıl", "yil", "yıllık", "yillik", "yıl bazında"}},
		{Locale: "en", Domain: DomainGrainSynonym, Key: "quarter", Terms: []string{"quarter", "quarters", "qtr"}},
		{Locale: "tr", Domain: DomainGrainSynonym, Key: "quarter", Terms: []string{"çeyrek", "ceyrek", "çeyreklik", "ceyreklik"}},
		{Locale: "en", Domain: DomainGrainSynonym, Key: "month", Terms: []string{"month", "months", "monthly", "per month", "by month"}},
		{Locale: "tr", Domain: DomainGrainSynonym, Key: "month", Terms: []string{"ay", "aylık", "aylik", "ay bazında"}},
		{Locale: "en", Domain: DomainGrainSynonym, Key: "day", Terms: []string{"day", "days", "daily", "per day", "by day"}},
		{Locale: "tr", Domain: DomainGrainSynonym, Key: "day", Terms: []string{"gün", "gun", "günlük", "gunluk", "günü", "gunu"}},
		{Locale: "en", Domain: DomainGrainSynonym, Key: "hour", Terms: []string{"hour", "hours", "hourly", "per hour", "by hour"}},
		{Locale: "tr", Domain: DomainGrainSynonym, Key: "hour", Terms: []string{"saat", "saatlik", "saatte", "saatli"}},

		// --- soft_delete (from routing/model_builder.go softDeleteColumnSynonyms) ---
		{Locale: "en", Domain: DomainSoftDelete, Key: "ts_deleted", Terms: []string{"deleted", "removed", "trashed", "erased", "soft delete", "soft-delete"}},
		{Locale: "tr", Domain: DomainSoftDelete, Key: "ts_deleted", Terms: []string{"silinen", "silinmiş", "silindi", "silinmis", "kaldırılan", "kaldirilan"}},
		{Locale: "en", Domain: DomainSoftDelete, Key: "ts_archived", Terms: []string{"archived", "deleted"}},
		{Locale: "tr", Domain: DomainSoftDelete, Key: "ts_archived", Terms: []string{"arşiv", "arsiv", "arşivlenmiş", "arsivlenmis", "silinen", "kaldırılan", "kaldirilan"}},
		{Locale: "en", Domain: DomainSoftDelete, Key: "bool_deleted", Terms: []string{"deleted", "removed", "archived"}},
		{Locale: "tr", Domain: DomainSoftDelete, Key: "bool_deleted", Terms: []string{"silinen", "silinmiş", "silinmis", "silindi", "kaldırılan", "kaldirilan"}},
		{Locale: "en", Domain: DomainSoftDelete, Key: "num_delete_flag", Terms: []string{"deleted", "delete flag"}},
		{Locale: "tr", Domain: DomainSoftDelete, Key: "num_delete_flag", Terms: []string{"silinen", "silme bayrağı", "silme bayragi"}},

		// --- intent_token (from routing/routing_budget.go) ---
		{Locale: "en", Domain: DomainIntentToken, Key: "count", Terms: []string{"count", "quantity"}},
		{Locale: "tr", Domain: DomainIntentToken, Key: "count", Terms: []string{"kaç", "adet"}},
		{Locale: "en", Domain: DomainIntentToken, Key: "total", Terms: []string{"total"}},
		{Locale: "tr", Domain: DomainIntentToken, Key: "total", Terms: []string{"toplam"}},
		{Locale: "en", Domain: DomainIntentToken, Key: "average", Terms: []string{"average"}},
		{Locale: "tr", Domain: DomainIntentToken, Key: "average", Terms: []string{"ortalama"}},
		{Locale: "en", Domain: DomainIntentToken, Key: "time_grain_mention", Terms: []string{"yesterday", "today", "week", "month", "year", "quarter", "daily", "hourly"}},
		{Locale: "tr", Domain: DomainIntentToken, Key: "time_grain_mention", Terms: []string{"dün", "bugün", "hafta", "ay", "yıl", "günlük", "saat", "dakika", "dk"}},
		{Locale: "tr", Domain: DomainIntentToken, Key: "ranking_top_n", Terms: []string{"ilk 10", "ilk 5", "en çok", "dakiler", "dakileri"}},

		// --- row_count (from semanticgen/generator.go countMetric) ---
		{Locale: "en", Domain: DomainRowCount, Key: "row_count", Terms: []string{"count", "total rows"}},
		{Locale: "tr", Domain: DomainRowCount, Key: "row_count", Terms: []string{"adet", "sayisi", "kac tane"}},
	}
}
