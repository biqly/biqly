package env

import (
	"testing"
)

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"empty", nil, false},
		{"bi_env production", map[string]string{"BI_ENV": "production"}, true},
		{"bi_env prod", map[string]string{"BI_ENV": "prod"}, true},
		{"bi_env development", map[string]string{"BI_ENV": "development"}, false},
		{"app_env production", map[string]string{"APP_ENV": "production"}, true},
		{"kubernetes", map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1"}, true},
		{"bi_env wins over empty k8s marker", map[string]string{"BI_ENV": "development", "KUBERNETES_SERVICE_HOST": ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BI_ENV", "")
			t.Setenv("APP_ENV", "")
			t.Setenv("KUBERNETES_SERVICE_HOST", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := IsProduction(); got != tt.want {
				t.Errorf("IsProduction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHSTSEnabledDefault(t *testing.T) {
	tests := []struct {
		name    string
		biEnv   string
		origins []string
		want    bool
	}{
		{"dev no origins", "development", nil, false},
		{"production", "production", nil, true},
		{"https webauthn origin", "development", []string{"https://app.example.com"}, true},
		{"http only", "development", []string{"http://localhost:3333"}, false},
		{"app_env alone not enough", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BI_ENV", tt.biEnv)
			if got := HSTSEnabledDefault(tt.origins...); got != tt.want {
				t.Errorf("HSTSEnabledDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
