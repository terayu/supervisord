package ini

import (
	"os"
	"testing"
)

func load(content string) *Ini {
	cfg := NewIni()
	cfg.LoadString(content)
	return cfg
}

func value(t *testing.T, cfg *Ini, section, key string) string {
	t.Helper()
	v, err := cfg.GetValue(section, key)
	if err != nil {
		t.Fatalf("[%s] %s: %v", section, key, err)
	}
	return v
}

// Backslashes are ordinary characters, as in python's RawConfigParser, so a
// Windows path can be written as it is.
func TestBackslashIsLiteral(t *testing.T) {
	cases := map[string]string{
		`D:\log\app.log`:          `D:\log\app.log`,
		`C:\temp\new\x.log`:       `C:\temp\new\x.log`,
		`D:\\log\\app.log`:        `D:\\log\\app.log`,
		`a\;b`:                    `a\;b`,
		`\a\b\f\n\r\t\v\x0041\0`:  `\a\b\f\n\r\t\v\x0041\0`,
		`\101`:                    `\101`,
		`D:\log\ ; a comment`:     `D:\log\`,
		`D:\log dir\ # a comment`: `D:\log dir\`,
	}
	for in, want := range cases {
		cfg := load("[s]\nkey = " + in + "\n")
		if got := value(t, cfg, "s", "key"); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// Behaviour that does not depend on backslash escapes and must not change.
func TestInlineComments(t *testing.T) {
	cfg := load("[s]\na = x ; comment\nb = x;y\nc = x # comment\nd = x#y\n")
	for key, want := range map[string]string{"a": "x", "b": "x;y", "c": "x", "d": "x#y"} {
		if got := value(t, cfg, "s", key); got != want {
			t.Errorf("%s: got %q, want %q", key, got, want)
		}
	}
}

func TestContinuationLines(t *testing.T) {
	cfg := load("[s]\na = one \\\ntwo \\\nthree\nb = next\n")
	if got := value(t, cfg, "s", "a"); got != "one two three" {
		t.Errorf("a: got %q", got)
	}
	if got := value(t, cfg, "s", "b"); got != "next" {
		t.Errorf("b: got %q", got)
	}
	// an even number of trailing backslashes is not a continuation
	cfg = load("[s]\na = D:\\\\log\\\\\nb = next\n")
	if got := value(t, cfg, "s", "a"); got != `D:\\log\\` {
		t.Errorf("a: got %q", got)
	}
	if got := value(t, cfg, "s", "b"); got != "next" {
		t.Errorf("b: got %q", got)
	}
}

func TestIndentedContinuation(t *testing.T) {
	cfg := load("[s]\na = one\n  two\n  three\nb = next\n")
	if got := value(t, cfg, "s", "a"); got != "one\ntwo\nthree" {
		t.Errorf("a: got %q", got)
	}
	if got := value(t, cfg, "s", "b"); got != "next" {
		t.Errorf("b: got %q", got)
	}
}

func TestEnvironmentVariables(t *testing.T) {
	os.Setenv("INI_TEST_VAR", "value")
	os.Unsetenv("INI_TEST_UNSET")
	cfg := load("[s]\na = ${INI_TEST_VAR}/x\nb = ${INI_TEST_UNSET:-fallback}\n")
	if got := value(t, cfg, "s", "a"); got != "value/x" {
		t.Errorf("a: got %q", got)
	}
	if got := value(t, cfg, "s", "b"); got != "fallback" {
		t.Errorf("b: got %q", got)
	}
}

func TestSectionsAndMerge(t *testing.T) {
	cfg := NewIni()
	cfg.LoadString("[program:a]\ncommand = one\n[program:b]\ncommand = two\n")
	cfg.LoadString("[program:a]\ndirectory = /tmp\n")
	if got := value(t, cfg, "program:a", "command"); got != "one" {
		t.Errorf("command: got %q", got)
	}
	if got := value(t, cfg, "program:a", "directory"); got != "/tmp" {
		t.Errorf("directory: got %q", got)
	}
	if n := len(cfg.Sections()); n != 2 {
		t.Errorf("got %d sections, want 2", n)
	}
}
