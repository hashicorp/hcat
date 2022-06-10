package tfunc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/hcat"
	idep "github.com/hashicorp/hcat/internal/dependency"

	"github.com/stretchr/testify/assert"
)

func init() {
	idep.FileQuerySleepTime = 50 * time.Millisecond
}

func Test_NewFileQuery(t *testing.T) {
	cases := []struct {
		name string
		i    string
		id   string
		err  bool
	}{
		{
			"empty",
			"",
			"error prevents object creation, so no ID test",
			true,
		},
		{
			"path",
			"path",
			"file(path)",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act, err := idep.NewFileQuery(tc.i)
			if err != nil {
				if !tc.err {
					t.Fatal(err)
				}
				return
			}
			act.Stop()
			assert.Equal(t, tc.id, act.ID())
		})
	}
}

func Test_FileQuery_Fetch(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("hello world"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		i    string
		exp  string
		err  bool
	}{
		{
			"non_existent",
			"/not/a/real/path/ever",
			"",
			true,
		},
		{
			"contents",
			f.Name(),
			"hello world",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := idep.NewFileQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}

			act, _, err := d.Fetch(nil)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			assert.Equal(t, tc.exp, act)
		})
	}

	t.Run("stops", func(t *testing.T) {
		f, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())

		d, err := idep.NewFileQuery(f.Name())
		if err != nil {
			t.Fatal(err)
		}

		errCh := make(chan error, 1)
		go func() {
			for {
				_, _, err := d.Fetch(nil)
				if err != nil {
					errCh <- err
					return
				}
			}
		}()

		d.Stop()

		select {
		case err := <-errCh:
			if err != idep.ErrStopped {
				t.Fatal(err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("did not stop")
		}
	})

	syncWriteFile := func(name string, data []byte, perm os.FileMode) error {
		f, err := os.OpenFile(name,
			os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_SYNC, perm)
		if err == nil {
			_, err = f.Write(data)
			if err1 := f.Close(); err1 != nil && err == nil {
				err = err1
			}
		}
		return err
	}
	t.Run("fires_changes", func(t *testing.T) {
		f, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := syncWriteFile(f.Name(), []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())

		d, err := idep.NewFileQuery(f.Name())
		if err != nil {
			t.Fatal(err)
		}

		dataCh := make(chan interface{}, 1)
		errCh := make(chan error, 1)
		go func() {
			for {
				data, _, err := d.Fetch(nil)
				if err != nil {
					errCh <- err
					return
				}
				dataCh <- data
			}
		}()
		defer d.Stop()

		select {
		case err := <-errCh:
			t.Fatal(err)
		case <-dataCh:
		}

		if err := syncWriteFile(f.Name(), []byte("goodbye"), 0644); err != nil {
			t.Fatal(err)
		}

		select {
		case err := <-errCh:
			t.Fatal(err)
		case data := <-dataCh:
			assert.Equal(t, "goodbye", data)
		}
	})
}

func Test_FileQuery_String(t *testing.T) {
	cases := []struct {
		name string
		i    string
		exp  string
	}{
		{
			"path",
			"path",
			"file(path)",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			d, err := idep.NewFileQuery(tc.i)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.exp, d.String())
		})
	}
}

func Test_writeToFile(t *testing.T) {
	// Use current user and its primary group for input
	currentUser, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	currentUsername := currentUser.Username
	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		t.Fatal(err)
	}
	currentGroupName := currentGroup.Name

	cases := []struct {
		name        string
		filePath    string
		content     string
		username    string
		groupName   string
		permissions string
		flags       string
		expectation string
		wantErr     bool
	}{
		{
			"writeToFile_without_flags",
			"",
			"after",
			currentUsername,
			currentGroupName,
			"0644",
			"",
			"after",
			false,
		},
		{
			"writeToFile_with_different_file_permissions",
			"",
			"after",
			currentUsername,
			currentGroupName,
			"0666",
			"",
			"after",
			false,
		},
		{
			"writeToFile_with_append",
			"",
			"after",
			currentUsername,
			currentGroupName,
			"0644",
			`"append"`,
			"beforeafter",
			false,
		},
		{
			"writeToFile_with_newline",
			"",
			"after",
			currentUsername,
			currentGroupName,
			"0644",
			`"newline"`,
			"after\n",
			false,
		},
		{
			"writeToFile_with_append_and_newline",
			"",
			"after",
			currentUsername,
			currentGroupName,
			"0644",
			`"append,newline"`,
			"beforeafter\n",
			false,
		},
		{
			"writeToFile_default_owner",
			"",
			"after",
			"",
			"",
			"0644",
			"",
			"after",
			false,
		},
		{
			"writeToFile_provide_uid_gid",
			"",
			"after",
			currentUser.Uid,
			currentUser.Gid,
			"0644",
			"",
			"after",
			false,
		},
		{
			"writeToFile_provide_just_gid",
			"",
			"after",
			"",
			currentUser.Gid,
			"0644",
			"",
			"after",
			false,
		},
		{
			"writeToFile_provide_just_uid",
			"",
			"after",
			currentUser.Uid,
			"",
			"0644",
			"",
			"after",
			false,
		},
		{
			"writeToFile_create_directory",
			"demo/testing.tmp",
			"after",
			currentUsername,
			currentGroupName,
			"0644",
			"",
			"after",
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(outDir)

			var outputFilePath string
			if tc.filePath == "" {
				outputFile, err := ioutil.TempFile(outDir, "")
				if err != nil {
					t.Fatal(err)
				}
				_, err = outputFile.WriteString("before")
				if err != nil {
					t.Fatal(err)
				}
				outputFilePath = outputFile.Name()
			} else {
				outputFilePath = outDir + "/" + tc.filePath
			}

			templateContent := fmt.Sprintf(
				"{{ \"%s\" | writeToFile \"%s\" \"%s\" \"%s\" \"%s\"  %s}}",
				tc.content, outputFilePath, tc.username, tc.groupName, tc.permissions, tc.flags)
			ti := hcat.TemplateInput{
				Contents: templateContent,
			}
			tpl := newTemplate(ti)

			output, err := tpl.Execute(nil)
			if (err != nil) != tc.wantErr {
				t.Errorf("writeToFile() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			// Compare generated file content with the expectation.
			// The function should generate an empty string to the output.
			_generatedFileContent, err := ioutil.ReadFile(outputFilePath)
			generatedFileContent := string(_generatedFileContent)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(""), output) {
				t.Errorf("writeToFile() template = %v, want empty string", output)
			}
			if generatedFileContent != tc.expectation {
				t.Errorf("writeToFile() got = %v, want %v", generatedFileContent, tc.expectation)
			}
			// Assert output file permissions
			sts, err := os.Stat(outputFilePath)
			if err != nil {
				t.Fatal(err)
			}
			p_u, err := strconv.ParseUint(tc.permissions, 8, 32)
			if err != nil {
				t.Fatal(err)
			}
			perm := os.FileMode(p_u)
			if sts.Mode() != perm {
				t.Errorf("writeToFile() wrong permissions got = %v, want %v", perm, tc.permissions)
			}

			stat := sts.Sys().(*syscall.Stat_t)
			u := strconv.FormatUint(uint64(stat.Uid), 10)
			g := strconv.FormatUint(uint64(stat.Gid), 10)
			if u != currentUser.Uid || g != currentUser.Gid {
				t.Errorf("writeToFile() owner = %v:%v, wanted %v:%v", u, g, currentUser.Uid, currentUser.Gid)
			}
		})
	}
}
