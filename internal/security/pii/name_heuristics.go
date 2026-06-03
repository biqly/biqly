package pii

import "strings"

// nameKeyword associates a PII type with the column-name keywords that imply
// it. Keywords are matched on token boundaries ("zip" never matches "ip").
// Order matters: the first matching entry wins when several types apply.
type nameKeyword struct {
	piiType  string
	keywords []string
}

var nameKeywords = []nameKeyword{
	{TypeEmail, []string{"email", "e_mail", "e_posta", "eposta", "mail"}},
	{TypeTCKimlikNo, []string{"tckn", "tc_kimlik", "tc_kimlik_no", "tc", "identity", "national_id", "kimlik"}},
	{TypeIBAN, []string{"iban", "bank_account"}},
	{TypeCreditCardLike, []string{"card_number", "cc_number", "credit_card", "pan", "kart_no"}},
	{TypeIPAddress, []string{"ip_address", "ip_addr", "ip"}},
	{TypePhone, []string{"phone", "tel", "telephone", "telefon", "gsm", "mobile", "cep"}},
	{TypeAddress, []string{"address", "adres", "addr"}}, //nolint:misspell // "adres" is the Turkish keyword
}

// detectTypeFromName returns the PII type implied by the column name, or ""
// when no keyword matches.
func detectTypeFromName(columnName string) string {
	norm := normalizeName(columnName)
	if norm == "" {
		return ""
	}
	padded := "_" + norm + "_"
	for _, entry := range nameKeywords {
		for _, kw := range entry.keywords {
			if strings.Contains(padded, "_"+kw+"_") {
				return entry.piiType
			}
		}
	}
	return ""
}

// normalizeName lowercases the column name and collapses separators to "_"
// so keyword matching works on token boundaries (e.g. "E-Posta", "userEmail").
func normalizeName(name string) string {
	var b strings.Builder
	b.Grow(len(name) + 4)
	prevLower := false
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
			if prevLower {
				b.WriteByte('_') // camelCase boundary
			}
			b.WriteRune(r - 'A' + 'a')
			prevLower = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevLower = r >= 'a' && r <= 'z'
		default:
			b.WriteByte('_')
			prevLower = false
		}
	}
	return strings.Trim(b.String(), "_")
}
