package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/component-base/logs"

	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd"
	e "github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	g "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	"github.com/openshift/origin/test/extended/util"
	framework "k8s.io/kubernetes/test/e2e/framework"

	// Import test packages from this module
	_ "github.com/openshift/ipfailover-tests-extension/e2e"
)

func main() {
	// Initialize test framework flags (required for kubeconfig, provider, etc.)
	util.InitStandardFlags()
	framework.AfterReadingAllFlags(&framework.TestContext)

	logs.InitLogs()
	defer logs.FlushLogs()

	registry := e.NewRegistry()
	ext := e.NewExtension("openshift", "payload", "ipfailover")

	// Register test suites (parallel, serial, disruptive, all)
	registerSuites(ext)

	// Build test specs from Ginkgo
	// Note: ModuleTestsOnly() is applied by default, which filters out /vendor/ and k8s.io/kubernetes tests
	allSpecs, err := g.BuildExtensionTestSpecsFromOpenShiftGinkgoSuite()
	if err != nil {
		panic(fmt.Sprintf("couldn't build extension test specs from ginkgo: %+v", err.Error()))
	}

	// Filter to only include tests from this module's e2e/ directory
	// Excludes tests from /go/pkg/mod/ (module cache) and /vendor/
	componentSpecs := allSpecs.Select(func(spec *et.ExtensionTestSpec) bool {
		for _, loc := range spec.CodeLocations {
			// Include tests from local e2e/ directory (not from module cache or vendor)
			if strings.Contains(loc, "/e2e/") && !strings.Contains(loc, "/go/pkg/mod/") && !strings.Contains(loc, "/vendor/") {
				return true
			}
		}
		return false
	})

	// Initialize test framework before all tests
	// Note: In OTE, we don't need the OTP-specific InitTest() call
	// The framework initialization is handled by the test execution itself

	// Process all specs
	componentSpecs.Walk(func(spec *et.ExtensionTestSpec) {
		// Apply platform filters based on Platform: labels
		for label := range spec.Labels {
			if strings.HasPrefix(label, "Platform:") {
				platformName := strings.TrimPrefix(label, "Platform:")
				spec.Include(et.PlatformEquals(platformName))
			}
		}

		// Apply platform filters based on [platform:xxx] in test names
		re := regexp.MustCompile(`\[platform:([a-z]+)\]`)
		if match := re.FindStringSubmatch(spec.Name); match != nil {
			platform := match[1]
			spec.Include(et.PlatformEquals(platform))
		}

		// Set lifecycle to Blocking (tests should fail the suite if they fail)
		spec.Lifecycle = et.LifecycleBlocking
	})

	// Add filtered component specs to extension
	ext.AddSpecs(componentSpecs)

	registry.Register(ext)

	root := &cobra.Command{
		Long: "Ipfailover Tests",
	}

	root.AddCommand(cmd.DefaultExtensionCommands(registry)...)

	if err := func() error {
		return root.Execute()
	}(); err != nil {
		os.Exit(1)
	}
}

// registerSuites registers test suites with proper categorization
func registerSuites(ext *e.Extension) {
	suites := []e.Suite{
		{
			Name: "ipfailover/conformance/serial",
			Parents: []string{
				"openshift/conformance/serial",
			},
			Description: "Serial conformance tests (must run sequentially)",
			Qualifiers: []string{
				`name.contains("[Serial]") && !name.contains("[Disruptive]")`,
			},
		},
		{
			Name:        "ipfailover/disruptive",
			Parents:     []string{"openshift/disruptive"},
			Description: "Disruptive tests (may affect cluster state)",
			Qualifiers: []string{
				`name.contains("[Disruptive]")`,
			},
		},
		{
			Name:        "ipfailover/all",
			Description: "All ipfailover tests",
			// No qualifiers means all tests from this extension will be included
		},
	}

	for _, suite := range suites {
		ext.AddSuite(suite)
	}
}
