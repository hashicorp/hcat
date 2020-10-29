package tfunc

import (
	"fmt"
	"strings"

	socktmpl "github.com/hashicorp/go-sockaddr/template"
)

// sockaddr wraps go-sockaddr templating
func sockaddr(args ...string) (string, error) {
	t := fmt.Sprintf("{{ %s }}", strings.Join(args, " "))
	k, err := socktmpl.Parse(t)
	if err != nil {
		return "", err
	}
	return k, nil
}
