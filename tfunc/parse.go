package tfunc

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// parseBool parses a string into a boolean
func parseBool(s string) (bool, error) {
	if s == "" {
		return false, nil
	}

	result, err := strconv.ParseBool(s)
	if err != nil {
		return false, errors.Wrap(err, "parseBool")
	}
	return result, nil
}

// parseFloat parses a string into a base 10 float
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0.0, nil
	}

	result, err := strconv.ParseFloat(s, 10)
	if err != nil {
		return 0, errors.Wrap(err, "parseFloat")
	}
	return result, nil
}

// parseInt parses a string into a base 10 int
func parseInt(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	result, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "parseInt")
	}
	return result, nil
}

// parseJSON returns a structure for valid JSON
func parseJSON(s string) (interface{}, error) {
	if s == "" {
		return map[string]interface{}{}, nil
	}

	var data interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, err
	}
	return data, nil
}

// parseUint parses a string into a base 10 int
func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}

	result, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "parseUint")
	}
	return result, nil
}

// parseYAML returns a structure for valid YAML
func parseYAML(s string) (interface{}, error) {
	if s == "" {
		return map[string]interface{}{}, nil
	}

	var data interface{}
	if err := yaml.Unmarshal([]byte(s), &data); err != nil {
		return nil, err
	}
	return data, nil
}
