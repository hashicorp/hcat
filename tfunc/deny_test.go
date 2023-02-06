// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import "testing"

func TestDeny(t *testing.T) {
	v, err := DenyFunc()
	if v != "" {
		t.Errorf("bad return string: '%v'", v)
	}
	if err != disabledErr {
		t.Errorf("bad error: %v", err)
	}
}
