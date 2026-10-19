package router

import (
	_ "embed"
	"os"
	"path/filepath"
	"sync"

	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

//go:embed testdata/router/ipfailover.yaml
var ipfailoverTemplate []byte

var (
	ipfailoverTemplateOnce sync.Once
	ipfailoverTemplateDir  string
	ipfailoverTemplateFile string
	ipfailoverTemplateErr  error
)

// ipfailoverTemplatePath writes the embedded ipfailover template to a temporary
// file the first time it is called and returns the path to that file.
//
// The template is materialized lazily rather than from an init function. This
// package is linked into a standalone test extension binary, and openshift-tests
// runs that binary with subcommands such as "info", "list" and "images" while it
// is discovering tests. Those subcommands run from a working directory that is
// not guaranteed to be writable, so creating files from init made every one of
// them panic before any test could run. Deferring the work to the point of use
// keeps discovery free of filesystem side effects.
func ipfailoverTemplatePath() string {
	ipfailoverTemplateOnce.Do(func() {
		var dir string
		dir, ipfailoverTemplateErr = os.MkdirTemp("", "ipfailover-testdata-")
		if ipfailoverTemplateErr != nil {
			return
		}
		path := filepath.Join(dir, "ipfailover.yaml")
		if ipfailoverTemplateErr = os.WriteFile(path, ipfailoverTemplate, 0644); ipfailoverTemplateErr != nil {
			return
		}
		ipfailoverTemplateDir = dir
		ipfailoverTemplateFile = path
	})
	o.Expect(ipfailoverTemplateErr).NotTo(o.HaveOccurred(), "could not materialize the embedded ipfailover template")
	return ipfailoverTemplateFile
}

// CleanupTestData removes the temporary directory created by
// ipfailoverTemplatePath. It is registered through
// ExtensionTestSpecs.AddAfterAll, which runs it at the end of whichever process
// executed the tests. A multi-test run re-execs this binary with "run-test"
// once per spec, so each of those processes removes its own directory.
//
// Ginkgo's AfterSuite is deliberately not used here: the extension's Ginkgo
// adapter calls ClearBeforeAndAfterSuiteNodes, so a suite hook would be
// discarded and never run.
//
// Processes that never materialized the template, such as the parent of a
// multi-test run, leave the directory empty and are a no-op. Guarding on the
// directory rather than deriving it from the file path matters: the directory
// of an empty path is ".", which would delete the working directory.
func CleanupTestData() {
	if ipfailoverTemplateDir == "" {
		return
	}
	if err := os.RemoveAll(ipfailoverTemplateDir); err != nil {
		e2e.Logf("failed to remove ipfailover test data directory %s: %v", ipfailoverTemplateDir, err)
	}
}
