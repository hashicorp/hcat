package hcat

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
)

var (
	runExamples = flag.Bool("egs", false, "Run example tests")
	consuladdr  string
)

func TestMain(m *testing.M) {
	flag.Parse()
	cleanup := func() {}
	if *runExamples {
		consuladdr, cleanup = testConsulSetup()
	}
	retCode := m.Run()
	cleanup() // can't defer w/ os.Exit
	os.Exit(retCode)
}

// support for running consul as part of integration testing
func testConsulSetup() (string, func()) {
	var err error
	origStderr := os.Stderr
	os.Stderr, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err != nil {
		os.Stderr = origStderr
	}
	consul, err := testutil.NewTestServerConfig(
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
