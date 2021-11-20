package tfunc

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
)

// writeToFile writes the content to a file allowing the setting of username,
// group name, permissions and flags to select appending mode or add a newline.
//
// If not set, username and groupname default to those running the process and
// perms default to 0755 (plus umask). If only one of username or groupname are
// given it will be used for both.
//
// For example:
//   key "key/path" | writeToFile "/file/path.txt"
//   key "key/path" | writeToFile "/file/path.txt" "my-user" "my-group" "0644" "append"
//   key "key/path" | writeToFile "/file/path.txt" "my-user" "my-group" "0644" "append,newline"
//
func writeToFile(path string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("writeToFile: no content provided")
	}
	// content is always last arg
	content, args := args[len(args)-1], args[:len(args)-1]
	// Parse arguments
	var shouldAppend, shouldAddNewLine bool
	var perms, username, groupname string
	var err error
	for _, arg := range args {
		switch {
		case strings.Contains(arg, "append") || strings.Contains(arg, "newline"):
			shouldAppend = strings.Contains(arg, "append")
			shouldAddNewLine = strings.Contains(arg, "newline")
		case isPerm(arg):
			perms = arg
		case userExists(arg):
			username = arg
			if groupname == "" && groupExists(arg) {
				groupname = arg
			}
		case groupExists(arg):
			groupname = arg
		default:
			return "", fmt.Errorf("writeToFile: bad argument, %v", arg)
		}
	}

	perm := os.FileMode(0755) // default
	if perms != "" {
		p_u, err := strconv.ParseUint(perms, 8, 32)
		if err != nil {
			return "", err
		}
		perm = os.FileMode(p_u)
	}

	// Write to file
	var f *os.File
	if shouldAppend {
		f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, perm)
		if err != nil {
			return "", err
		}
	} else {
		f, err = os.Create(path)
		if err != nil {
			return "", err
		}
	}
	defer f.Close()

	writingContent := []byte(content)
	if shouldAddNewLine {
		writingContent = append(writingContent, []byte("\n")...)
	}
	if _, err = f.Write(writingContent); err != nil {
		return "", err
	}

	if username != "" {
		// Change ownership and permissions
		u, err := user.Lookup(username)
		if err != nil {
			return "", err
		}
		g, err := user.LookupGroup(groupname)
		if err != nil {
			return "", err
		}
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(g.Gid)
		err = os.Chown(path, uid, gid)
		if err != nil {
			return "", err
		}
	}

	err = os.Chmod(path, perm)
	if err != nil {
		return "", err
	}

	return "", nil
}

func isPerm(perms string) bool {
	_, err := strconv.ParseUint(perms, 8, 32)
	return err == nil
}

func userExists(username string) bool {
	_, err := user.Lookup(username)
	return err == nil
}

func groupExists(groupname string) bool {
	_, err := user.LookupGroup(groupname)
	return err == nil
}
