package spec

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidateArgValues checks resolved argument values against the constraints
// declared in the spec. It runs at execution time, after arguments have been
// parsed and defaults applied, but before any step executes. The values map is
// the resolved arg map (every value is a string).
func ValidateArgValues(s *CommandSpec, values map[string]any) error {
	for _, p := range s.Args.Positional {
		v := stringValue(values[p.Name])
		if v == "" && !p.IsRequired() {
			continue
		}
		if err := validatePositionalValue(p, v); err != nil {
			return err
		}
	}
	for _, f := range s.Args.Flags {
		if f.GetType() == "bool" {
			continue
		}
		v := stringValue(values[f.Name])
		if v == "" && !f.IsRequired() {
			continue
		}
		if err := validateFlagValue(f, v); err != nil {
			return err
		}
	}
	return nil
}

func validatePositionalValue(p PositionalArg, value string) error {
	return validateStringValue("argument "+p.Name, value, p.Enum, p.Pattern, p.MinLength, p.MaxLength)
}

func validateFlagValue(f FlagArg, value string) error {
	label := "flag --" + f.Name
	if f.GetType() == "int" {
		return validateIntValue(label, value, f.Min, f.Max)
	}
	return validateStringValue(label, value, f.Enum, f.Pattern, f.MinLength, f.MaxLength)
}

func validateStringValue(label, value string, enum []string, pattern string, minLen, maxLen *int) error {
	if len(enum) > 0 {
		found := false
		for _, e := range enum {
			if e == value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s must be one of: %s (got %q)", label, strings.Join(enum, ", "), value)
		}
	}
	if pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("%s has an invalid pattern %q: %w", label, pattern, err)
		}
		if !re.MatchString(value) {
			return fmt.Errorf("%s must match pattern %s (got %q)", label, pattern, value)
		}
	}
	n := utf8.RuneCountInString(value)
	if minLen != nil && n < *minLen {
		return fmt.Errorf("%s must be at least %d character(s) (got %d)", label, *minLen, n)
	}
	if maxLen != nil && n > *maxLen {
		return fmt.Errorf("%s must be at most %d character(s) (got %d)", label, *maxLen, n)
	}
	return nil
}

func validateIntValue(label, value string, minV, maxV *int) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s must be an integer (got %q)", label, value)
	}
	if minV != nil && n < *minV {
		return fmt.Errorf("%s must be >= %d (got %d)", label, *minV, n)
	}
	if maxV != nil && n > *maxV {
		return fmt.Errorf("%s must be <= %d (got %d)", label, *maxV, n)
	}
	return nil
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return DefaultToString(v)
}

// DefaultToString renders a flag default value to the string used in templates
// and validation. Integral numbers are formatted as integers (JSON unmarshals
// numbers to float64, whose %v rendering uses scientific notation for large
// values — undesirable for int flags like ports or byte counts).
func DefaultToString(v any) string {
	if v == nil {
		return ""
	}
	if f, ok := v.(float64); ok && f == math.Trunc(f) && !math.IsInf(f, 0) {
		return strconv.FormatInt(int64(f), 10)
	}
	return fmt.Sprintf("%v", v)
}
