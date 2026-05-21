package ai

import (
	"strings"

	"github.com/biqly/biqly/internal/i18n"
)

// DetectQuestionLocale picks tr vs en from the natural-language question so
// table routing uses the matching embedding vectors and localized metadata.
func DetectQuestionLocale(question string) i18n.Locale {
	q := strings.TrimSpace(question)
	if q == "" {
		return i18n.DefaultLocale
	}
	if hasTurkishLetters(q) {
		return i18n.LocaleTR
	}
	lower := " " + strings.ToLower(q) + " "
	for _, w := range turkishQuestionWords {
		if strings.Contains(lower, w) {
			return i18n.LocaleTR
		}
	}
	return i18n.LocaleEN
}

func hasTurkishLetters(s string) bool {
	for _, r := range s {
		switch r {
		case 'ı', 'İ', 'ş', 'Ş', 'ğ', 'Ğ', 'ü', 'Ü', 'ö', 'Ö', 'ç', 'Ç':
			return true
		}
	}
	return false
}

var turkishQuestionWords = []string{
	" kaç ", " kaç?", " adet ", " göster", " listele", " toplam ",
	" ortalama ", " günlük", " aylık", " yıllık", " dün", " bugün", " geçen ",
	" son ", " filtre", " göre", " arasında", " tarih", " müşteri", " sipariş",
	" satış", " ürün", " tweet", " kullanıcı", " sayısı", " miktar",
}
