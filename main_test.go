package hcat

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/hcat/internal/test"
)

var (
	RunExamples = flag.Bool("egs", false, "Run example tests")
	Consuladdr  string
)

func TestMain(m *testing.M) {
	flag.Parse()
	cleanup := func() {}
	if *RunExamples {
		Consuladdr, cleanup = testConsulSetup()
	}
	retCode := m.Run()
	cleanup() // can't defer w/ os.Exit
	os.Exit(retCode)
}

// support for running consul as part of integration testing
func testConsulSetup() (string, func()) {
	var err error
	origStderr := os.Stderr
	os.Stderr, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0o666)
	if err != nil {
		os.Stderr = origStderr
	}
	tb := &test.TestingTB{}
	consul, err := testutil.NewTestServerConfigT(tb,
		func(c *testutil.TestServerConfig) {
			c.LogLevel = "error"
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		})
	if err != nil {
		log.Fatalf("failed to start consul server: %v", err)
	}
	os.Stderr = origStderr
	return consul.HTTPAddr, func() { consul.Stop() }
}
