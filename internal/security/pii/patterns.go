package pii

import (
	"net/netip"
	"regexp"
	"slices"
	"strings"
)

// Supported PII type identifiers. They mirror the values allowed by the
// chk_columns_pii_type constraint in migration 038a.
const (
	TypeEmail          = "email"
	TypePhone          = "phone"
	TypeIBAN           = "iban"
	TypeTCKimlikNo     = "tc_kimlik_no"
	TypeAddress        = "address"
	TypeIPAddress      = "ip_address"
	TypeCreditCardLike = "credit_card_like"
)

// AllTypes lists every supported PII type in detection priority order.
var AllTypes = []string{
	TypeEmail,
	TypeTCKimlikNo,
	TypeIBAN,
	TypeCreditCardLike,
	TypeIPAddress,
	TypePhone,
	TypeAddress,
}

// IsValidType reports whether t is a recognized PII type identifier.
func IsValidType(t string) bool {
	return slices.Contains(AllTypes, t)
}

var (
	emailRe       = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	trPhoneRe     = regexp.MustCompile(`^(\+90|0)?[0-9]{10}$`)
	intlPhoneRe   = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
	trIBANRe      = regexp.MustCompile(`^TR[0-9]{24}$`)
	genericIBANRe = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$`)
	tcknRe        = regexp.MustCompile(`^[1-9][0-9]{10}$`)
	cardDigitsRe  = regexp.MustCompile(`^[0-9]{13,19}$`)
)

// valueMatcher reports whether a single sample value looks like the given PII
// type. Address has no value pattern; it is handled by the detector via the
// name heuristic plus a free-text length check.
type valueMatcher func(string) bool

var valueMatchers = map[string]valueMatcher{
	TypeEmail:          matchEmail,
	TypePhone:          matchPhone,
	TypeIBAN:           matchIBAN,
	TypeTCKimlikNo:     matchTCKN,
	TypeIPAddress:      matchIPAddress,
	TypeCreditCardLike: matchCreditCard,
	TypeAddress:        matchAddressText,
}

func matchEmail(v string) bool {
	return emailRe.MatchString(strings.TrimSpace(v))
}

func matchPhone(v string) bool {
	c := compactNumeric(v)
	return trPhoneRe.MatchString(c) || intlPhoneRe.MatchString(c)
}

func matchIBAN(v string) bool {
	c := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(v), " ", ""))
	return trIBANRe.MatchString(c) || genericIBANRe.MatchString(c)
}

func matchTCKN(v string) bool {
	c := strings.TrimSpace(v)
	return tcknRe.MatchString(c) && validTCKN(c)
}

func matchIPAddress(v string) bool {
	_, err := netip.ParseAddr(strings.TrimSpace(v))
	return err == nil
}

func matchCreditCard(v string) bool {
	c := compactNumeric(v)
	return cardDigitsRe.MatchString(c) && luhnValid(c)
}

// matchAddressText is a weak corroboration signal: free text long enough to
// plausibly hold a postal address. The detector only applies it when the
// column name already implies an address.
func matchAddressText(v string) bool {
	return len(strings.TrimSpace(v)) > 20
}

// compactNumeric strips common phone/card formatting characters so the
// numeric regexes can match values like "+90 (555) 123-45-67".
func compactNumeric(v string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '(', ')', '.':
			return -1
		}
		return r
	}, strings.TrimSpace(v))
}

// validTCKN validates the Turkish national identity number checksum.
func validTCKN(s string) bool {
	if len(s) != 11 {
		return false
	}
	var d [11]int
	for i := range 11 {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		d[i] = int(c - '0')
	}
	if d[0] == 0 {
		return false
	}
	odd := d[0] + d[2] + d[4] + d[6] + d[8]
	even := d[1] + d[3] + d[5] + d[7]
	check10 := (odd*7 - even) % 10
	if check10 < 0 {
		check10 += 10
	}
	if d[9] != check10 {
		return false
	}
	sum := 0
	for i := range 10 {
		sum += d[i]
	}
	return d[10] == sum%10
}

// luhnValid validates a digit string with the Luhn checksum used by payment
// card numbers.
func luhnValid(s string) bool {
	sum := 0
	double := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		n := int(c - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}
