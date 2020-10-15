package tfunc

import (
	"fmt"
	"regexp"
	"strings"
)

// Indent prefixes each line of a string with the specified number of spaces
func indent(spaces int, s string) (string, error) {
	if spaces < 0 {
		return "", fmt.Errorf("indent value must be a positive integer")
	}
	var output, prefix []byte
	var sp bool
	var size int
	prefix = []byte(strings.Repeat(" ", spaces))
	sp = true
	for _, c := range []byte(s) {
		if sp && c != '\n' {
			output = append(output, prefix...)
			size += spaces
		}
		output = append(output, c)
		sp = c == '\n'
		size++
	}
	return string(output[:size]), nil
}

// join is a version of strings.Join that can be piped
func join(sep string, a []string) (string, error) {
	return strings.Join(a, sep), nil
}

// split is a version of strings.Split that can be piped
func split(sep, s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}, nil
	}
	return strings.Split(s, sep), nil
}

// TrimSpace is a version of strings.TrimSpace that can be piped
func trimSpace(s string) (string, error) {
	return strings.TrimSpace(s), nil
}

// replaceAll replaces all occurrences of a value in a string with the given
// replacement value.
func replaceAll(f, t, s string) (string, error) {
	return strings.Replace(s, f, t, -1), nil
}

// regexReplaceAll replaces all occurrences of a regular expression with
// the given replacement value.
func regexReplaceAll(re, pl, s string) (string, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return "", err
	}
	return compiled.ReplaceAllString(s, pl), nil
}

// regexMatch returns true or false if the string matches
// the given regular expression
func regexMatch(re, s string) (bool, error) {
	compiled, err := regexp.Compile(re)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(s), nil
}
