package router

import (
	_ "embed"
	"os"
	"path/filepath"
	"sync"

	o "github.com/onsi/gomega"
)

//go:embed testdata/router/ipfailover.yaml
var ipfailoverTemplate []byte

var (
	ipfailoverTemplateOnce sync.Once
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
		ipfailoverTemplateFile = path
	})
	o.Expect(ipfailoverTemplateErr).NotTo(o.HaveOccurred(), "could not materialize the embedded ipfailover template")
	return ipfailoverTemplateFile
}
