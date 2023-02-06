// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfunc

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/hcat"
	idep "github.com/hashicorp/hcat/internal/dependency"
)

// fileFunc returns the contents of the file and monitors a file for changes
func fileFunc(recall hcat.Recaller) interface{} {
	return func(s string) (string, error) {
		if len(s) == 0 {
			return "", nil
		}
		d, err := idep.NewFileQuery(s)
		if err != nil {
			return "", err
		}

		if value, ok := recall(d); ok {
			if value == nil {
				return "", nil
			}
			return value.(string), nil
		}

		return "", nil
	}
}

// writeToFile writes the content to a file with optional flags for
// permissions, username (or UID), group name (or GID), and to select appending
// mode or add a newline.
//
// The username and group name fields can be left blank to default to the
// current user and group.
//
// For example:
//   key "my/key/path" | writeToFile "/my/file/path.txt" "" "" "0644"
//   key "my/key/path" | writeToFile "/my/file/path.txt" "100" "1000" "0644"
//   key "my/key/path" | writeToFile "/my/file/path.txt" "my-user" "my-group" "0644"
//   key "my/key/path" | writeToFile "/my/file/path.txt" "my-user" "my-group" "0644" "append"
//   key "my/key/path" | writeToFile "/my/file/path.txt" "my-user" "my-group" "0644" "append,newline"
//
func writeToFile(path, username, groupName, permissions string, args ...string) (string, error) {
	// Parse arguments
	flags := ""
	if len(args) == 2 {
		flags = args[0]
	}
	content := args[len(args)-1]

	p_u, err := strconv.ParseUint(permissions, 8, 32)
	if err != nil {
		return "", err
	}
	perm := os.FileMode(p_u)

	// Write to file
	var f *os.File
	shouldAppend := strings.Contains(flags, "append")
	if shouldAppend {
		f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, perm)
		if err != nil {
			return "", err
		}
	} else {
		dirPath := filepath.Dir(path)

		if _, err := os.Stat(dirPath); err != nil {
			err := os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				return "", err
			}
		}

		f, err = os.Create(path)
		if err != nil {
			return "", err
		}
	}
	defer f.Close()

	writingContent := []byte(content)
	shouldAddNewLine := strings.Contains(flags, "newline")
	if shouldAddNewLine {
		writingContent = append(writingContent, []byte("\n")...)
	}
	if _, err = f.Write(writingContent); err != nil {
		return "", err
	}

	// Change ownership and permissions
	var uid int
	var gid int
	if err != nil {
		return "", err
	}

	if username == "" {
		uid = os.Getuid()
	} else {
		var convErr error
		u, err := user.Lookup(username)
		if err != nil {
			// Check if username string is already a UID
			uid, convErr = strconv.Atoi(username)
			if convErr != nil {
				return "", err
			}
		} else {
			uid, _ = strconv.Atoi(u.Uid)
		}
	}

	if groupName == "" {
		gid = os.Getgid()
	} else {
		var convErr error
		g, err := user.LookupGroup(groupName)
		if err != nil {
			gid, convErr = strconv.Atoi(groupName)
			if convErr != nil {
				return "", err
			}
		} else {
			gid, _ = strconv.Atoi(g.Gid)
		}
	}

	// Avoid the chown call altogether if using current user and group.
	if username != "" || groupName != "" {
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
