package env

import (
	"testing"
	"time"
)

func TestString(t *testing.T) {
	t.Setenv("X_STR", "v")
	if got := String("X_STR", "def"); got != "v" {
		t.Errorf("got %q", got)
	}
	if got := String("X_UNSET", "def"); got != "def" {
		t.Errorf("got %q", got)
	}
}

func TestInt(t *testing.T) {
	t.Setenv("X_INT", "notanint")
	if got := Int("X_INT", 7); got != 7 {
		t.Errorf("invalid int should use default, got %d", got)
	}
	t.Setenv("X_INT", "42")
	if got := Int("X_INT", 7); got != 42 {
		t.Errorf("got %d", got)
	}
}

func TestPositiveInt(t *testing.T) {
	t.Setenv("X_PI", "0")
	if got := PositiveInt("X_PI", 5); got != 5 {
		t.Errorf("zero should use default, got %d", got)
	}
	t.Setenv("X_PI", "-3")
	if got := PositiveInt("X_PI", 5); got != 5 {
		t.Errorf("negative should use default, got %d", got)
	}
	t.Setenv("X_PI", "3")
	if got := PositiveInt("X_PI", 5); got != 3 {
		t.Errorf("got %d", got)
	}
}

func TestNonNegativeInt(t *testing.T) {
	t.Setenv("X_NN", "0")
	if got := NonNegativeInt("X_NN", 5); got != 0 {
		t.Errorf("zero should be accepted, got %d", got)
	}
	t.Setenv("X_NN", "-1")
	if got := NonNegativeInt("X_NN", 5); got != 5 {
		t.Errorf("negative should use default, got %d", got)
	}
}

func TestBool(t *testing.T) {
	for _, v := range []string{"1", "true", "YES", "on"} {
		t.Setenv("X_B", v)
		if !Bool("X_B", false) {
			t.Errorf("%q should be true", v)
		}
	}
	for _, v := range []string{"0", "false", "No", "off"} {
		t.Setenv("X_B", v)
		if Bool("X_B", true) {
			t.Errorf("%q should be false", v)
		}
	}
	t.Setenv("X_B", "maybe")
	if !Bool("X_B", true) {
		t.Error("unrecognized value should use default")
	}
}

func TestDuration(t *testing.T) {
	t.Setenv("X_D", "bad")
	if got := Duration("X_D", time.Second); got != time.Second {
		t.Errorf("invalid duration should use default, got %s", got)
	}
	t.Setenv("X_D", "2m")
	if got := Duration("X_D", time.Second); got != 2*time.Minute {
		t.Errorf("got %s", got)
	}
}

func TestPositiveDuration(t *testing.T) {
	t.Setenv("X_PD", "0s")
	if got := PositiveDuration("X_PD", time.Second); got != time.Second {
		t.Errorf("zero should use default, got %s", got)
	}
	t.Setenv("X_PD", "3s")
	if got := PositiveDuration("X_PD", time.Second); got != 3*time.Second {
		t.Errorf("got %s", got)
	}
}

func TestCSV(t *testing.T) {
	t.Setenv("X_CSV", " a , ,b,")
	got := CSV("X_CSV")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v", got)
	}
	t.Setenv("X_CSV", "  ")
	if got := CSV("X_CSV"); got != nil {
		t.Errorf("blank should be nil, got %v", got)
	}
}

func TestCSVDefault(t *testing.T) {
	def := []string{"d"}
	t.Setenv("X_CSVD", "")
	if got := CSVDefault("X_CSVD", def); len(got) != 1 || got[0] != "d" {
		t.Errorf("unset should return default, got %v", got)
	}
	t.Setenv("X_CSVD", "x,y")
	if got := CSVDefault("X_CSVD", def); len(got) != 2 {
		t.Errorf("set should parse, got %v", got)
	}
}
