// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

// secretFunc returns or accumulates secret dependencies from Vault.
func secretFunc(recall hcat.Recaller) interface{} {
	return func(s ...string) (interface{}, error) {
		if len(s) == 0 {
			return nil, nil
		}

		path, rest := s[0], s[1:]
		data := make(map[string]interface{})
		for _, str := range rest {
			if len(str) == 0 {
				continue
			}
			parts := strings.SplitN(str, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("not k=v pair %q", str)
			}

			k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			data[k] = v
		}

		var d dep.Dependency
		var err error

		isReadQuery := len(rest) == 0
		if isReadQuery {
			d, err = idep.NewVaultReadQuery(path)
		} else {
			d, err = idep.NewVaultWriteQuery(path, data)
		}

		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			return value.(*dep.Secret), nil
		}

		return nil, nil
	}
}

// secretsFunc returns or accumulates a list of secret dependencies from Vault.
func secretsFunc(recall hcat.Recaller) interface{} {
	return func(s string) ([]string, error) {
		var result []string

		if len(s) == 0 {
			return result, nil
		}

		d, err := idep.NewVaultListQuery(s)
		if err != nil {
			return nil, err
		}

		if value, ok := recall(d); ok {
			result = value.([]string)
			return result, nil
		}

		return result, nil
	}
}
