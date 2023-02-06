// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"os"
	"strings"
)

// envFunc returns a function which checks the value of an environment variable.
// Invokers can specify their own environment, which takes precedences over any
// real environment variables
func envFunc(env []string) func(string) (string, error) {
	return func(s string) (string, error) {
		for _, e := range env {
			split := strings.SplitN(e, "=", 2)
			k, v := split[0], split[1]
			if k == s {
				return v, nil
			}
		}
		return os.Getenv(s), nil
	}
}

// envOrDefaultFunc returns a function which checks the value of an
// environment variable. Invokers can specify their own environment, which
// takes precedences over any real environment variables. If an environment
// variable is found, the value of that variable will be used. This includes
// empty values. Otherwise, the default will be used instead.
func envOrDefaultFunc(env []string) func(string, string) (string, error) {
	return func(s string, def string) (string, error) {
		for _, e := range env {
			split := strings.SplitN(e, "=", 2)
			k, v := split[0], split[1]
			if k == s {
				return v, nil
			}
		}
		val, isPresent := os.LookupEnv(s)
		if isPresent {
			return val, nil
		}
		return def, nil
	}
}
