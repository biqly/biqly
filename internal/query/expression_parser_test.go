package query

import (
	"testing"
)

func TestValidateExpression(t *testing.T) {
	tests := []struct {
		expr    string
		isValid bool
	}{
		// Valid expressions
		{"[total_amount] - [discount]", true},
		{"COALESCE([amount], 0)", true},
		{"CASE WHEN price > 0 THEN price ELSE 0 END", true},
		{"1 + 2 * 3 / 4", true},
		{"(a + b) * c", true},
		{"UPPER(CONCAT(first_name, ' ', last_name))", true},
		{"amount IS NULL", true},
		{"amount IS NOT NULL", true},
		{"price BETWEEN 10 AND 20", true},
		{"status IN ('active', 'pending')", true},
		{"name LIKE 'John%'", true},

		// Invalid expressions (banned keywords / syntax)
		{"1; DROP TABLE users", false},
		{"(SELECT * FROM users)", false},
		{"exec xp_cmdshell", false},
		{"SELECT 1", false},
		{"INSERT INTO users VALUES (1)", false},
		{"UPDATE users SET name = 'foo'", false},
		{"DELETE FROM users", false},
		{"DROP TABLE users", false},
		{"ALTER TABLE users ADD COLUMN age INT", false},
		{"CREATE TABLE users (id INT)", false},

		// Comments (forbidden)
		{"amount -- some comment", false},
		{"amount /* some comment */ + tax", false},

		// Semicolons (forbidden)
		{"amount; tax", false},

		// Invalid syntax
		{"(amount", false},
		{"COALESCE(amount,", false},
		{"CASE WHEN price > 0 THEN price END", true},
		{"CASE WHEN price > 0 THEN price", false},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			err := ValidateExpression(tc.expr)
			if tc.isValid && err != nil {
				t.Errorf("expected valid for %q, got error: %v", tc.expr, err)
			} else if !tc.isValid && err == nil {
				t.Errorf("expected invalid for %q, got no error", tc.expr)
			}
		})
	}
}
