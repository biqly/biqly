package semantic

import "testing"

func TestContainsDMLKeywordSubstringFalsePositive(t *testing.T) {
	// Column names containing DML keywords as substrings must NOT trigger.
	safe := []string{
		"select_count",
		"users.delete_flag",
		"orders.updated_at",
		"CASE WHEN price > 0 THEN price ELSE 0 END",
	}
	for _, expr := range safe {
		if kw, hit := containsDMLKeyword(expr); hit {
			t.Errorf("expected %q to pass DML guard, matched %q", expr, kw)
		}
	}
}

func TestContainsDMLKeywordDetectsActualKeywords(t *testing.T) {
	bad := []string{
		"SELECT * FROM users",
		"INSERT INTO x VALUES (1)",
		"UPDATE x SET y=1",
		"DELETE FROM x",
		"DROP TABLE x",
		"CREATE INDEX y",
	}
	for _, expr := range bad {
		if _, hit := containsDMLKeyword(expr); !hit {
			t.Errorf("expected %q to be rejected by DML guard", expr)
		}
	}
}

func TestContainsDMLKeywordSkipsStringLiterals(t *testing.T) {
	expr := `CASE WHEN status = 'select' THEN 1 ELSE 0 END`
	if kw, hit := containsDMLKeyword(expr); hit {
		t.Errorf("DML keyword inside string literal should be ignored, matched %q", kw)
	}
}

func TestContainsDMLKeywordSkipsLineComments(t *testing.T) {
	expr := "amount -- INSERT was once considered here\n + tax"
	if kw, hit := containsDMLKeyword(expr); hit {
		t.Errorf("DML keyword inside line comment should be ignored, matched %q", kw)
	}
}

func TestContainsDMLKeywordSkipsBlockComments(t *testing.T) {
	expr := "amount /* DELETE history note */ + tax"
	if kw, hit := containsDMLKeyword(expr); hit {
		t.Errorf("DML keyword inside block comment should be ignored, matched %q", kw)
	}
}
