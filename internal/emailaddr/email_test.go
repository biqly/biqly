package emailaddr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:funlen
func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		want    string
		wantErr bool
	}{
		{
			name:    "empty email",
			email:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "spaces only",
			email:   "   ",
			want:    "",
			wantErr: true,
		},
		{
			name:    "valid standard email",
			email:   "user@example.com",
			want:    "user@example.com",
			wantErr: false,
		},
		{
			name:    "valid standard email with uppercase",
			email:   "User@Example.com",
			want:    "user@example.com",
			wantErr: false,
		},
		{
			name:    "valid standard email with spaces",
			email:   "  user@example.com  ",
			want:    "user@example.com",
			wantErr: false,
		},
		{
			name:    "googlemail to gmail",
			email:   "foo@googlemail.com",
			want:    "foo@gmail.com",
			wantErr: false,
		},
		{
			name:    "gmail with plus tag",
			email:   "foo+bar@gmail.com",
			want:    "foo@gmail.com",
			wantErr: false,
		},
		{
			name:    "gmail with dots",
			email:   "f.o.o.b.a.r@gmail.com",
			want:    "foobar@gmail.com",
			wantErr: false,
		},
		{
			name:    "gmail with dots and tags",
			email:   "f.o.o+b.a.r@gmail.com",
			want:    "foo@gmail.com",
			wantErr: false,
		},
		{
			name:    "googlemail with dots and tags",
			email:   "f.o.o+b.a.r@googlemail.com",
			want:    "foo@gmail.com",
			wantErr: false,
		},
		{
			name:    "non-gmail with plus tag remains untouched",
			email:   "foo+bar@example.com",
			want:    "foo+bar@example.com",
			wantErr: false,
		},
		{
			name:    "non-gmail with dots remains untouched",
			email:   "f.o.o@example.com",
			want:    "f.o.o@example.com",
			wantErr: false,
		},
		{
			name:    "invalid email - missing @",
			email:   "example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid email - double @",
			email:   "user@@example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid email - empty local",
			email:   "@example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid email - empty domain",
			email:   "user@",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid email - contains <",
			email:   "user<@example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid email - contains >",
			email:   "user>@example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "gmail dot trick empty local portion",
			email:   ".+tag@gmail.com",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{
			name:  "empty email",
			email: "",
			want:  "",
		},
		{
			name:  "invalid email format",
			email: "invalid-no-at",
			want:  "***",
		},
		{
			name:  "invalid email format - empty domain",
			email: "foo@",
			want:  "***",
		},
		{
			name:  "invalid email format - empty local",
			email: "@bar",
			want:  "***",
		},
		{
			name:  "one character local",
			email: "a@example.com",
			want:  "*@example.com",
		},
		{
			name:  "two characters local",
			email: "ab@example.com",
			want:  "a*@example.com",
		},
		{
			name:  "three characters local",
			email: "abc@example.com",
			want:  "a*c@example.com",
		},
		{
			name:  "many characters local",
			email: "abcdef@example.com",
			want:  "a****f@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mask(tt.email)
			assert.Equal(t, tt.want, got)
		})
	}
}
