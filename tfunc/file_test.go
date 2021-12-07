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

	"github.com/hashicorp/hcat"
)

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
