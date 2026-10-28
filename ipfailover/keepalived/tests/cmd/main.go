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
	ipfailovertests "github.com/openshift/images/ipfailover-tests-extension/e2e"
)

func main() {
	// Initialize test framework flags (required for kubeconfig, provider, etc.)
	util.InitStandardFlags()

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
	})

	// Every test here deploys keepalived on the host network and claims a VIP,
	// so the whole extension shares one set of environment constraints. The
	// BeforeEach still re-checks these at runtime, but declaring them here keeps
	// openshift-tests from scheduling the specs at all on clusters that cannot
	// run them, rather than creating a namespace per spec only to skip.
	componentSpecs.
		Include(et.Or(
			// "none" covers baremetal clusters installed without the platform.
			et.PlatformEquals("none"),
			et.PlatformEquals("baremetal"),
			et.PlatformEquals("vsphere"),
			et.PlatformEquals("openstack"),
			et.PlatformEquals("nutanix"),
		)).
		// ipfailover has no single-stack IPv6 support. Dual stack is fine.
		Exclude(et.NetworkStackEquals("ipv6")).
		// The tests reach for cluster-scoped resources that a HyperShift guest
		// does not have. This replaces the NonHyperShiftHOST name fragment,
		// which never excluded anything.
		Exclude(et.TopologyEquals("External")).
		// Excludes MicroShift and single-node. openshift-tests reports
		// MicroShift as platform "none", which the include above matches, so
		// the platform list alone does not keep these tests off it. MicroShift
		// has no config.openshift.io Infrastructure API for the BeforeEach to
		// read, and neither topology has the two worker nodes ipfailover needs.
		//
		// SingleReplica is the value openshift-tests actually sends: AddTopology
		// takes a configv1.TopologyMode, and determineEnvironmentFlags defaults
		// MicroShift to SingleReplica because it cannot read the clusterState.
		// The "microshift" literal is what the extension framework documents on
		// the --topology flag, so exclude it too in case another orchestrator
		// sends it.
		Exclude(et.TopologyEquals("SingleReplica")).
		Exclude(et.TopologyEquals("microshift"))

	// Remove the temporary directory holding the materialized test data once
	// every spec in this process has finished.
	componentSpecs.AddAfterAll(ipfailovertests.CleanupTestData)

	// Add filtered component specs to extension
	ext.AddSpecs(componentSpecs)

	registry.Register(ext)

	root := &cobra.Command{
		Long: "Ipfailover Tests",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize test context after flags are parsed but before tests run
			framework.AfterReadingAllFlags(&framework.TestContext)
		},
	}

	root.AddCommand(cmd.DefaultExtensionCommands(registry)...)

	if err := root.Execute(); err != nil {
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
			// Every test here is [Serial], including the two that are also
			// [Disruptive]. Those two disrupt only the ipfailover deployment in
			// their own namespace, not the cluster, so they belong in the serial
			// suite rather than openshift/disruptive.
			Qualifiers: []string{
				`name.contains("[Serial]")`,
			},
		},
		{
			// Deliberately has no parent. openshift/disruptive is only run by
			// AWS and GCP workflows, and this extension excludes both platforms,
			// so parenting here meant the two tests had no reachable CI job at
			// all. Kept as a standalone suite for running them on their own.
			Name:        "ipfailover/disruptive",
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
