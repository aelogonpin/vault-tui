package tui

import "testing"

func TestParseValue(t *testing.T) {
	cases := []struct {
		in   string
		want interface{}
	}{
		{"hello", "hello"},
		{"hello world", "hello world"},
		{"", ""},
		{"42", float64(42)},
		{`"42"`, "42"},
		{"true", true},
		{"false", false},
		{"null", nil},
		{`"true"`, "true"},
	}
	for _, c := range cases {
		got := parseValue(c.in)
		if got != c.want {
			t.Errorf("parseValue(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestToDisplayStringRoundTripsPlainStrings(t *testing.T) {
	// A plain string that isn't JSON-shaped must display unquoted and
	// round-trip through parseValue unchanged.
	for _, s := range []string{"admin", "hello world", "https://example.com", "s3cr3t!P@ss"} {
		disp := toDisplayString(s)
		if disp != s {
			t.Errorf("toDisplayString(%q) = %q, want unquoted passthrough", s, disp)
		}
		if got := parseValue(disp); got != s {
			t.Errorf("round-trip broke: parseValue(toDisplayString(%q)) = %#v", s, got)
		}
	}
}

func TestToDisplayStringQuotesJSONLookingStrings(t *testing.T) {
	// Regression guard: a *string* secret whose literal value happens to
	// look like JSON (a bare number/bool/null) must round-trip as a string,
	// not silently flip type to a JSON number/bool when the editor re-saves
	// it untouched. toDisplayString achieves this by quoting on display.
	for _, s := range []string{"12345", "true", "false", "null", "3.14"} {
		disp := toDisplayString(s)
		if disp == s {
			t.Errorf("toDisplayString(%q) = %q, expected it to be quoted to stay a string on save", s, disp)
		}
		got := parseValue(disp)
		gotStr, ok := got.(string)
		if !ok || gotStr != s {
			t.Errorf("round-trip broke: parseValue(toDisplayString(%q)) = %#v, want string %q", s, got, s)
		}
	}
}

func TestToDisplayStringForNonStringValues(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{float64(42), "42"},
		{true, "true"},
		{nil, "null"},
	}
	for _, c := range cases {
		if got := toDisplayString(c.in); got != c.want {
			t.Errorf("toDisplayString(%#v) = %q, want %q", c.in, got, c.want)
		}
		// And it must parse back to an equivalent value.
		if got := parseValue(toDisplayString(c.in)); got != c.in {
			t.Errorf("round-trip broke for %#v: got %#v", c.in, got)
		}
	}
}

func TestClassifyValue(t *testing.T) {
	cases := []struct {
		in   string
		want valueKind
	}{
		{"", kindText},
		{"admin", kindText},
		{"hello world", kindText},
		{"42", kindJSONNumber},
		{`"42"`, kindJSONString},
		{"true", kindJSONBool},
		{"null", kindJSONNull},
		{`{"a":1}`, kindJSONObject},
		{`["a","b"]`, kindJSONArray},
	}
	for _, c := range cases {
		if got := classifyValue(c.in); got != c.want {
			t.Errorf("classifyValue(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
