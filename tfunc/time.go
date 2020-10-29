package tfunc

import (
	"fmt"
	"strconv"
	"time"
)

// now is function that represents the current time in UTC. This is here
// primarily for the tests to override times.
var now = func() time.Time { return time.Now().UTC() }

// timestamp returns the current UNIX timestamp in UTC. If an argument is
// specified, it will be used to format the timestamp.
func timestamp(s ...string) (string, error) {
	switch len(s) {
	case 0:
		return now().Format(time.RFC3339), nil
	case 1:
		if s[0] == "unix" {
			return strconv.FormatInt(now().Unix(), 10), nil
		}
		return now().Format(s[0]), nil
	default:
		return "", fmt.Errorf("timestamp: wrong number of arguments, "+
			"expected 0 or 1, but got %d", len(s))
	}
}
