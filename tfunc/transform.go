package tfunc

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// base64Decode decodes the given string as a base64 string, returning an error
// if it fails.
func base64Decode(s string) (string, error) {
	v, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", errors.Wrap(err, "base64Decode")
	}
	return string(v), nil
}

// base64Encode encodes the given value into a string represented as base64.
func base64Encode(s string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(s)), nil
}

// base64URLDecode decodes the given string as a URL-safe base64 string.
func base64URLDecode(s string) (string, error) {
	v, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", errors.Wrap(err, "base64URLDecode")
	}
	return string(v), nil
}

// base64URLEncode encodes the given string to be URL-safe.
func base64URLEncode(s string) (string, error) {
	return base64.URLEncoding.EncodeToString([]byte(s)), nil
}

// sha256Hex return the sha256 hex of a string
func sha256Hex(item string) (string, error) {
	h := sha256.New()
	h.Write([]byte(item))
	output := hex.EncodeToString(h.Sum(nil))
	return output, nil
}

// toLower converts the given string (usually by a pipe) to lowercase.
func toLower(s string) (string, error) {
	return strings.ToLower(s), nil
}

// toUpper converts the given string (usually by a pipe) to uppercase.
func toUpper(s string) (string, error) {
	return strings.ToUpper(s), nil
}

// toTitle converts the given string (usually by a pipe) to titlecase.
func toTitle(s string) (string, error) {
	return strings.Title(s), nil
}

// toJSON converts the given structure into a deeply nested JSON string.
func toJSON(i interface{}) (string, error) {
	result, err := json.Marshal(i)
	if err != nil {
		return "", errors.Wrap(err, "toJSON")
	}
	return string(bytes.TrimSpace(result)), err
}

// toJSONPretty converts the given structure into a deeply nested pretty JSON
// string.
func toJSONPretty(i interface{}) (string, error) {
	result, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "toJSONPretty")
	}
	return string(bytes.TrimSpace(result)), err
}

// toYAML converts the given structure into a deeply nested YAML string.
func toYAML(m map[string]interface{}) (string, error) {
	result, err := yaml.Marshal(m)
	if err != nil {
		return "", errors.Wrap(err, "toYAML")
	}
	return string(bytes.TrimSpace(result)), nil
}

// toTOML converts the given structure into a deeply nested TOML string.
func toTOML(m map[string]interface{}) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := toml.NewEncoder(buf)
	if err := enc.Encode(m); err != nil {
		return "", errors.Wrap(err, "toTOML")
	}
	result, err := ioutil.ReadAll(buf)
	if err != nil {
		return "", errors.Wrap(err, "toTOML")
	}
	return string(bytes.TrimSpace(result)), nil
}
