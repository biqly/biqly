package pii

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectTypeFromName(t *testing.T) {
	cases := []struct {
		name     string
		expected string
	}{
		{"email", TypeEmail},
		{"customer_email", TypeEmail},
		{"E-Posta", TypeEmail},
		{"userEmail", TypeEmail},
		{"mail_address", TypeEmail}, // email keyword wins over address
		{"phone", TypePhone},
		{"gsm_no", TypePhone},
		{"cep_telefonu", TypePhone},
		{"mobile_number", TypePhone},
		{"iban", TypeIBAN},
		{"bank_account", TypeIBAN},
		{"tc_kimlik_no", TypeTCKimlikNo},
		{"tckn", TypeTCKimlikNo},
		{"national_id", TypeTCKimlikNo},
		{"address", TypeAddress},
		{"billing_addr", TypeAddress},
		{"adres", TypeAddress}, //nolint:misspell // Turkish column name
		{"ip_address", TypeIPAddress},
		{"client_ip", TypeIPAddress},
		{"card_number", TypeCreditCardLike},
		{"cc_number", TypeCreditCardLike},
		// negatives: token boundaries must hold
		{"zip_code", ""},
		{"shipment_id", ""},
		{"description", ""},
		{"panel_id", ""},
		{"total_amount", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, detectTypeFromName(tc.name))
		})
	}
}

func TestValueMatchers(t *testing.T) {
	cases := []struct {
		piiType string
		value   string
		match   bool
	}{
		{TypeEmail, "john.doe@example.com", true},
		{TypeEmail, "j+filter@sub.domain.co", true},
		{TypeEmail, "not-an-email", false},
		{TypeEmail, "missing@tld", false},

		{TypePhone, "05551234567", true},
		{TypePhone, "+905551234567", true},
		{TypePhone, "+90 (555) 123-45-67", true},
		{TypePhone, "+14155552671", true},
		{TypePhone, "12345", false},
		{TypePhone, "abc", false},

		{TypeIBAN, "TR330006100519786457841326", true},
		{TypeIBAN, "TR33 0006 1005 1978 6457 8413 26", true},
		{TypeIBAN, "DE89370400440532013000", true},
		{TypeIBAN, "XX123", false},

		{TypeTCKimlikNo, "10000000146", true},  // valid checksum
		{TypeTCKimlikNo, "10000000147", false}, // checksum fails
		{TypeTCKimlikNo, "01000000146", false}, // leading zero
		{TypeTCKimlikNo, "12345", false},

		{TypeIPAddress, "192.168.1.1", true},
		{TypeIPAddress, "::1", true},
		{TypeIPAddress, "2001:db8::ff00:42:8329", true},
		{TypeIPAddress, "999.999.999.999", false},
		{TypeIPAddress, "12345", false},

		{TypeCreditCardLike, "4111111111111111", true},
		{TypeCreditCardLike, "4111 1111 1111 1111", true},
		{TypeCreditCardLike, "4111111111111112", false}, // Luhn fails
		{TypeCreditCardLike, "1234", false},

		{TypeAddress, "Atatürk Mah. Cumhuriyet Cad. No:5 D:3 Kadıköy", true},
		{TypeAddress, "short", false},
	}
	for _, tc := range cases {
		t.Run(tc.piiType+"/"+tc.value, func(t *testing.T) {
			assert.Equal(t, tc.match, valueMatchers[tc.piiType](tc.value))
		})
	}
}

func TestDetectFromColumn_NameAndSamples(t *testing.T) {
	d := NewDetector(DefaultThreshold)

	results := d.DetectFromColumn("customer_email", []string{
		"a@example.com", "b@example.com", "c@example.com",
	})
	require.Len(t, results, 1)
	assert.Equal(t, TypeEmail, results[0].Type)
	assert.InDelta(t, 1.0, results[0].Confidence, 1e-9)
	assert.Equal(t, SourceNameSample, results[0].Source)
}

func TestDetectFromColumn_NameOnlyBelowThreshold(t *testing.T) {
	d := NewDetector(DefaultThreshold)

	// Name keyword alone scores 0.5 < 0.6: not flagged with default threshold.
	assert.Empty(t, d.DetectFromColumn("customer_email", nil))

	// A lower threshold flags it.
	low := NewDetector(0.4)
	results := low.DetectFromColumn("customer_email", nil)
	require.Len(t, results, 1)
	assert.Equal(t, TypeEmail, results[0].Type)
	assert.InDelta(t, 0.5, results[0].Confidence, 1e-9)
	assert.Equal(t, SourceName, results[0].Source)
}

func TestDetectFromColumn_PartialSampleMatch(t *testing.T) {
	d := NewDetector(DefaultThreshold)

	// Name match (0.5) + half the samples matching (0.25) = 0.75.
	results := d.DetectFromColumn("email", []string{
		"a@example.com", "b@example.com", "garbage", "123",
	})
	require.Len(t, results, 1)
	assert.Equal(t, TypeEmail, results[0].Type)
	assert.InDelta(t, 0.75, results[0].Confidence, 1e-9)
}

func TestDetectFromColumn_NullAndEmptySamplesIgnored(t *testing.T) {
	d := NewDetector(DefaultThreshold)

	// Empty/whitespace samples must not dilute the match ratio.
	results := d.DetectFromColumn("email", []string{
		"", "   ", "a@example.com", "\n", "b@example.com",
	})
	require.Len(t, results, 1)
	assert.InDelta(t, 1.0, results[0].Confidence, 1e-9)
}

func TestDetectFromColumn_AddressRequiresNameMatch(t *testing.T) {
	d := NewDetector(DefaultThreshold)
	longTexts := []string{
		"Atatürk Mah. Cumhuriyet Cad. No:5 D:3 Kadıköy/İstanbul",
		"Bağdat Caddesi No:101 Maltepe İstanbul 34840",
	}

	// Long free text alone must never flag as address.
	assert.Empty(t, d.DetectFromColumn("notes", longTexts))

	// Name match + long text corroboration flags it.
	results := d.DetectFromColumn("shipping_address", longTexts)
	require.Len(t, results, 1)
	assert.Equal(t, TypeAddress, results[0].Type)
	assert.InDelta(t, 1.0, results[0].Confidence, 1e-9)
}

func TestDetectFromColumn_NoFalsePositiveOnPlainData(t *testing.T) {
	d := NewDetector(DefaultThreshold)
	assert.Empty(t, d.DetectFromColumn("status", []string{"active", "inactive", "pending"}))
	assert.Empty(t, d.DetectFromColumn("amount", []string{"10.50", "99.99", "0.01"}))
	assert.Empty(t, d.DetectFromColumn("id", []string{"1", "2", "3"}))
}

func TestDetectFromColumn_SortedByConfidence(t *testing.T) {
	// TCKN values are also 11-digit numbers; a column named "tc" with valid
	// TCKNs must rank tc_kimlik_no first.
	d := NewDetector(0.1)
	results := d.DetectFromColumn("tc", []string{"10000000146"})
	require.NotEmpty(t, results)
	assert.Equal(t, TypeTCKimlikNo, results[0].Type)
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].Confidence, results[i].Confidence)
	}
}

func TestNewDetector_DefaultThresholdFallback(t *testing.T) {
	assert.InDelta(t, DefaultThreshold, NewDetector(0).Threshold(), 1e-9)
	assert.InDelta(t, DefaultThreshold, NewDetector(-1).Threshold(), 1e-9)
	assert.InDelta(t, 0.8, NewDetector(0.8).Threshold(), 1e-9)
}

func TestIsValidType(t *testing.T) {
	for _, typ := range AllTypes {
		assert.True(t, IsValidType(typ))
	}
	assert.False(t, IsValidType("ssn"))
	assert.False(t, IsValidType(""))
}
