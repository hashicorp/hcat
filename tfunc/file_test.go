package tfunc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"
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
	myUser, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	myUsername := myUser.Username
	myGroup, err := user.LookupGroupId(myUser.Gid)
	if err != nil {
		t.Fatal(err)
	}
	myGroupName := myGroup.Name

	cases := []struct {
		name        string
		content     string
		args        []string
		expectation string
		wantErr     bool
	}{
		{
			"writeToFile_plain",
			"after",
			[]string{},
			"after",
			false,
		},
		{
			"writeToFile_without_flags",
			"after",
			[]string{"0644"},
			"after",
			false,
		},
		{
			"writeToFile_with_different_file_permissions",
			"after",
			[]string{"0666"},
			"after",
			false,
		},
		{
			"writeToFile_with_username",
			"after",
			[]string{myUsername},
			"after",
			false,
		},
		{
			"writeToFile_with_user_group",
			"after",
			[]string{myUsername, myGroupName},
			"after",
			false,
		},
		{
			"writeToFile_with_user_group_perms",
			"after",
			[]string{myUsername, myGroupName, "0644"},
			"after",
			false,
		},
		{
			"writeToFile_with_append",
			"after",
			[]string{"0644", `append`},
			"beforeafter",
			false,
		},
		{
			"writeToFile_with_newline",
			"after",
			[]string{"0644", `newline`},
			"after\n",
			false,
		},
		{
			"writeToFile_with_all_order1",
			"after",
			[]string{myUsername, myGroupName, "0644", `append,newline`},
			"beforeafter\n",
			false,
		},
		{
			"writeToFile_with_all_order2",
			"after",
			[]string{"0644", myUsername, myGroupName, `append,newline`},
			"beforeafter\n",
			false,
		},
		{
			"writeToFile_with_all_order3",
			"after",
			[]string{`append,newline`, "0644", myUsername, myGroupName},
			"beforeafter\n",
			false,
		},
		{
			"writeToFile_with_all_order4",
			"after",
			[]string{myUsername, "0644", `append,newline`, myGroupName},
			"beforeafter\n",
			false,
		},
		{
			"writeToFile_with_all_order5",
			"after",
			[]string{myGroupName, "0644", `append,newline`, myUsername},
			"beforeafter\n",
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
			outputFile, err := ioutil.TempFile(outDir, "")
			if err != nil {
				t.Fatal(err)
			}
			outputFile.WriteString("before")

			templateContent := fmt.Sprintf(
				`{{ "%s" | writeToFile "%s"`, tc.content, outputFile.Name())
			if len(tc.args) > 0 {
				templateContent += ` "` + strings.Join(tc.args, `" "`) + `"`
			}
			templateContent += ` }}`

			ti := hcat.TemplateInput{
				Contents: templateContent,
			}
			tpl := newTemplate(ti)

			a, err := tpl.Execute(nil)
			if (err != nil) != tc.wantErr {
				t.Errorf("writeToFile() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			// Compare generated file content with the expectation.
			// The function should generate an empty string to the output.
			_generatedFileContent, err := ioutil.ReadFile(outputFile.Name())
			generatedFileContent := string(_generatedFileContent)
			if err != nil {
				t.Fatal(err)
			}
			if a != nil && !bytes.Equal([]byte(""), a) {
				t.Errorf("writeToFile() template = %v, want empty string", a)
			}
			if generatedFileContent != tc.expectation {
				t.Errorf("writeToFile() got = %#v, want %#v", generatedFileContent, tc.expectation)
			}
			// Assert output file permissions
			sts, err := outputFile.Stat()
			if err != nil {
				t.Fatal(err)
			}
			perms := "0755"
			for _, arg := range tc.args {
				if isPerm(arg) {
					perms = arg
				}
			}
			p_u, err := strconv.ParseUint(perms, 8, 32)
			if err != nil {
				t.Fatal(err)
			}
			perm := os.FileMode(p_u)
			if sts.Mode() != perm {
				t.Errorf("writeToFile() wrong permissions got = %v, want %v", perm, perms)
			}
		})
	}
}
