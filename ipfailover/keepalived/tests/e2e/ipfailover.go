package router

import (
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

func init() {
	// Write directly to stderr — no timing dependency on GinkgoWriter
	// At init time, g.GinkgoWriter is still os.Stdout, which would contaminate JSON output
	klog.SetOutput(os.Stderr)

	// Redirect REST client warnings directly to stderr
	rest.SetDefaultWarningHandler(
		rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{}),
	)

	// Set testsStarted flag to allow OTP util functions like oc.Run() to work
	exutil.WithCleanup(func() {})
}

// [Feature:Router] gives these tests Component Readiness ownership. OTE-run
// tests report JUnit suite "openshift-tests" uniformly, so the ci-test-mapping
// Suite:"Network_Edge" matcher never fires for them; the SIG matcher for
// "Networking / router" requires sig-network-edge together with Feature:Router,
// which this supplies. See openshift/router#871 for the same gap on Route tests.
var _ = g.Describe("[OTP][sig-network-edge][Feature:Router] Network_Edge", func() {
	defer g.GinkgoRecover()

	// Use NewCLI which creates a namespace for each test (like openshift-tests-private)
	oc := exutil.NewCLI("router-ipfailover")
	var HAInterfaces = "br-ex"

	g.BeforeEach(func() {
		g.By("Check platforms")
		// infrastructures.config.openshift.io lives in the API group this
		// extension's environment selectors already require (see
		// clusterConfigAPIGroup in cmd/main.go), so a cluster reaching this line
		// is expected to serve it. An error here is therefore a real failure --
		// timeout, permission, or throttling -- not an unsupported environment.
		// Fail instead of skipping: a skip would quietly drop all six tests and
		// still report the suite green. See openshift/router#871, which found
		// the equivalent skip-on-any-error swallowing registry errors in the
		// Route tests.
		infraOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "could not read the cluster's platform type")
		platformtype := strings.ToLower(strings.TrimSpace(infraOutput))

		platforms := map[string]bool{
			// 'None' also for Baremetal
			"none":      true,
			"baremetal": true,
			"vsphere":   true,
			"openstack": true,
			"nutanix":   true,
		}
		if !platforms[platformtype] {
			g.Skip(fmt.Sprintf("Skip for non-supported platform: %s", platformtype))
		}

		g.By("check whether there are two worker nodes present for testing hostnetwork")
		workerNodeCount, _ := exactNodeDetails(oc)
		if workerNodeCount < 2 {
			g.Skip("Skipping as we need atleast two worker nodes")
		}

		g.By("check the cluster has remote worker profile")
		// Listing nodes works on every cluster these tests are selected for, so
		// an error here is a real problem rather than an unsupported
		// environment. Fail instead of skipping: a skip would quietly drop all
		// six tests and still report the suite green.
		remoteWorkerLabel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o", "jsonpath={.items[*].metadata.labels.node\\.openshift\\.io/remote-worker}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "could not read the remote-worker node labels")
		if strings.Contains(remoteWorkerLabel, "true") {
			g.Skip("Skip as ipfailover currently doesn't support on remote-worker profile")
		}

		g.By("check whether the cluster is not ipv6 single stack")
		// Check for IPv6 single-stack by evaluating the full CIDR list
		ipStackType := checkIPStackType(oc)
		if ipStackType == "ipv6single" {
			g.Skip("Skip as ipfailover currently doesn't support ipv6 single stack")
		}

	})

	g.JustBeforeEach(func() {
		g.By("Check network type")
		// Get network type using oc command instead of compat_otp
		networkTypeOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("network", "cluster", "-o=jsonpath={.status.networkType}").Output()
		if err == nil {
			networkType := strings.ToLower(strings.TrimSpace(networkTypeOutput))
			if strings.Contains(networkType, "openshiftsdn") {
				HAInterfaces = "ens3"
			}
		}
	})

	//Author:hongli
	g.It("support to deploy ipfailover [Serial]", g.Label("OCP-41025"), ote.Informing(), func() {
		customTemp := ipfailoverTemplatePath()
		var (
			ipf = ipfailoverDescription{
				name:        "ipf-41025",
				namespace:   "",
				image:       "",
				HAInterface: HAInterfaces,
				template:    customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, ipf.namespace)
		unicastIPFailover(oc, ipf.namespace, ipf.name)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		podName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ensureIpfailoverMasterBackup(oc, ipf.namespace, podName)
	})

	//Author:mjoseph
	g.It("pod and service automatically switched over to standby when master fails [Serial][Disruptive]", g.Label("OCP-41027"), ote.Informing(), func() {
		customTemp := ipfailoverTemplatePath()
		var (
			ipf = ipfailoverDescription{
				name:        "ipf-41027",
				namespace:   "",
				image:       "",
				HAInterface: HAInterfaces,
				template:    customTemp,
			}
		)
		g.By("1. Get pull spec of ipfailover image from payload")
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("2. Create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, ipf.namespace)
		unicastIPFailover(oc, ipf.namespace, ipf.name)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		podNames := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ensureIpfailoverMasterBackup(oc, ipf.namespace, podNames)

		g.By("3. Set the HA virtual IP for the failover group")
		ipv4Address := getPodIP(oc, ipf.namespace, podNames[0])
		virtualIP := replaceIPOctet(ipv4Address, 3, "100")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VIRTUAL_IPS="+virtualIP)

		g.By("4. Verify the HA virtual ip ENV variable")
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		newPodName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		masterNode, _ := ensureIpfailoverMasterBackup(oc, ipf.namespace, newPodName)
		checkenv := pollReadPodData(oc, ipf.namespace, newPodName[0], "/usr/bin/env ", "OPENSHIFT_HA_VIRTUAL_IPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS=" + virtualIP))

		g.By("5. Find the primary and the secondary pod using the virtual IP")
		primaryPod := getVipOwnerPod(oc, ipf.namespace, newPodName, virtualIP)
		o.Expect(masterNode).To(o.ContainSubstring(primaryPod))

		g.By("6. Restarting the ipfailover primary pod")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ipf.namespace, "pod", primaryPod).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("6a. Wait for replica replacement after pod deletion")
		err = wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
			replicas, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "-n", ipf.namespace, ipf.name, "-o", "jsonpath={.status.readyReplicas}").Output()
			if err != nil {
				return false, err
			}
			return replicas == "2", nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7. Verify the virtual IP is floated onto the new MASTER node")
		newPodName1 := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		newMasterNode, _ := ensureIpfailoverMasterBackup(oc, ipf.namespace, newPodName1)
		waitForPrimaryPod(oc, ipf.namespace, newMasterNode, virtualIP)
	})

	//Author:mjoseph
	g.It("ipfailover configuration can be customized by ENV [Serial]", g.Label("OCP-41028"), ote.Informing(), func() {
		customTemp := ipfailoverTemplatePath()
		var (
			ipf = ipfailoverDescription{
				name:        "ipf-41028",
				namespace:   "",
				image:       "",
				HAInterface: HAInterfaces,
				template:    customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, ipf.namespace)
		unicastIPFailover(oc, ipf.namespace, ipf.name)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")

		g.By("set the HA virtual IP for the failover group")
		podNames := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ipv4Address := getPodIP(oc, ipf.namespace, podNames[0])
		virtualIP := replaceIPOctet(ipv4Address, 3, "100")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VIRTUAL_IPS="+virtualIP)

		g.By("set other ipfailover env varibales")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_CONFIG_NAME=ipfailover")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VIP_GROUPS=4")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_MONITOR_PORT=30061")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VRRP_ID_OFFSET=2")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_REPLICA_COUNT=3")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_USE_UNICAST=true`)
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_NFTABLES_RULE=OUTPUT`)
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_NOTIFY_SCRIPT=/etc/keepalive/mynotifyscript.sh`)
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_CHECK_SCRIPT=/etc/keepalive/mycheckscript.sh`)
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_PREEMPTION=preempt_delay 600`)
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_CHECK_INTERVAL=3")

		g.By("verify the HA virtual ip ENV variable")
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		newPodName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ensureIpfailoverMasterBackup(oc, ipf.namespace, newPodName)
		checkenv := pollReadPodData(oc, ipf.namespace, newPodName[0], "/usr/bin/env ", "OPENSHIFT_HA_VIRTUAL_IPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS=" + virtualIP))

		g.By("check the ipfailover configurations and verify the other ENV variables")
		result := describePodResource(oc, newPodName[0], ipf.namespace)
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_VIP_GROUPS:         4"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_CONFIG_NAME:        ipfailover"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_MONITOR_PORT:       30061"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_VRRP_ID_OFFSET:     2"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_REPLICA_COUNT:      3"))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_USE_UNICAST:        true`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_NFTABLES_RULE:      OUTPUT`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_NOTIFY_SCRIPT:      /etc/keepalive/mynotifyscript.sh`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_CHECK_SCRIPT:       /etc/keepalive/mycheckscript.sh`))
		o.Expect(result).To(o.ContainSubstring(`OPENSHIFT_HA_PREEMPTION:         preempt_delay 600`))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_CHECK_INTERVAL:     3"))
		o.Expect(result).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS:        " + virtualIP))
	})

	//Author:mjoseph
	g.It("ipfailover can support up to a maximum of 255 VIPs for the entire cluster [Serial]", g.Label("OCP-41029"), ote.Informing(), func() {
		// BeforeEach already asserted this same read succeeds, so failing hard
		// here too keeps a transient flake between the two calls from running
		// the test on Nutanix instead of correctly skipping it, which silently
		// proceeding on any error would have allowed.
		infraOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "could not read the cluster's platform type")
		if strings.ToLower(strings.TrimSpace(infraOutput)) == "nutanix" {
			g.Skip("This test will not work for Nutanix")
		}
		customTemp := ipfailoverTemplatePath()
		var (
			ipf = ipfailoverDescription{
				name:        "ipf-41029",
				namespace:   "",
				image:       "",
				HAInterface: HAInterfaces,
				template:    customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, ipf.namespace)
		unicastIPFailover(oc, ipf.namespace, ipf.name)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		podName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ensureIpfailoverMasterBackup(oc, ipf.namespace, podName)

		g.By("add some VIP configuration for the failover group")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VRRP_ID_OFFSET=0")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VIP_GROUPS=238")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_VIRTUAL_IPS=192.168.254.1-255`)

		g.By("verify from the ipfailover pod, the 255 VIPs are added")
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		newPodName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		checkenv := pollReadPodData(oc, ipf.namespace, newPodName[0], "/usr/bin/env ", "OPENSHIFT_HA_VIP_GROUPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIP_GROUPS=238"))
	})

	//Author:mjoseph
	g.It("preemption strategy for keepalived ipfailover [Serial][Disruptive]", g.Label("OCP-41030"), ote.Informing(), func() {
		customTemp := ipfailoverTemplatePath()
		var (
			ipf = ipfailoverDescription{
				name:        "ipf-41030",
				namespace:   "",
				image:       "",
				HAInterface: HAInterfaces,
				template:    customTemp,
			}
		)
		g.By("1. Get pull spec of ipfailover image from payload")
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("2. Create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, ipf.namespace)
		unicastIPFailover(oc, ipf.namespace, ipf.name)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		podName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ensureIpfailoverMasterBackup(oc, ipf.namespace, podName)

		g.By("3. Set the HA virtual IP for the failover group")
		podNames := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ipv4Address := getPodIP(oc, ipf.namespace, podNames[0])
		virtualIP := replaceIPOctet(ipv4Address, 3, "100")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, "OPENSHIFT_HA_VIRTUAL_IPS="+virtualIP)

		g.By("4. Verify the HA virtual ip ENV variable")
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		newPodName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		master, backup := ensureIpfailoverMasterBackup(oc, ipf.namespace, newPodName)
		checkenv := pollReadPodData(oc, ipf.namespace, newPodName[0], "/usr/bin/env ", "OPENSHIFT_HA_VIRTUAL_IPS")
		o.Expect(checkenv).To(o.ContainSubstring("OPENSHIFT_HA_VIRTUAL_IPS=" + virtualIP))
		checkenv1 := pollReadPodData(oc, ipf.namespace, newPodName[0], "/usr/bin/env ", "OPENSHIFT_HA_PREEMPTION")
		o.Expect(checkenv1).To(o.ContainSubstring("nopreempt"))

		g.By("5. Find the primary and the secondary pod")
		primaryPod := getVipOwnerPod(oc, ipf.namespace, newPodName, virtualIP)
		secondaryPod := slicingElement(primaryPod, newPodName)
		o.Expect(master).To(o.ContainSubstring(primaryPod))
		o.Expect(backup).To(o.ContainSubstring(secondaryPod[0]))

		g.By("6. Restarting the ipfailover primary pod")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ipf.namespace, "pod", primaryPod).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("7. Verify whether the other pod becomes primary and it has the VIP")
		waitForPrimaryPod(oc, ipf.namespace, secondaryPod[0], virtualIP)

		g.By("8. Now set the preemption delay timer of 120s for the failover group")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_PREEMPTION=preempt_delay 120`)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		newPodName1 := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		new_master, _ := ensureIpfailoverMasterBackup(oc, ipf.namespace, newPodName1)
		checkenv2 := pollReadPodData(oc, ipf.namespace, newPodName1[0], "/usr/bin/env ", "OPENSHIFT_HA_PREEMPTION")
		o.Expect(checkenv2).To(o.ContainSubstring("preempt_delay 120"))

		g.By("9. Again restart the ipfailover primary(master) pod")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", ipf.namespace, "pod", new_master).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("10. Verify the VIP is assigned after the preempt delay expires")
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		latestpods := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		// waiting till the preempt delay 120 seconds expires
		time.Sleep(125 * time.Second)
		// VRRP priority depends on node IP, so we cannot predict which pod
		// becomes master. Verify the VIP is held by one of the remaining pods.
		actualMaster := getVipOwnerPod(oc, ipf.namespace, latestpods, virtualIP)
		klog.Infof("After preempt delay, pod %v is the master holding VIP %v", actualMaster, virtualIP)
	})

	//Author:mjoseph
	g.It("Excluding the existing VRRP cluster ID from ipfailover deployments [Serial]", g.Label("OCP-49214"), ote.Informing(), func() {
		customTemp := ipfailoverTemplatePath()
		var (
			ipf = ipfailoverDescription{
				name:        "ipf-49214",
				namespace:   "",
				image:       "",
				HAInterface: HAInterfaces,
				template:    customTemp,
			}
		)

		g.By("get pull spec of ipfailover image from payload")
		ipf.image = getImagePullSpecFromPayload(oc, "keepalived-ipfailover")
		ipf.namespace = oc.Namespace()
		g.By("create ipfailover deployment and ensure one of pod enter MASTER state")
		ipf.create(oc, ipf.namespace)
		unicastIPFailover(oc, ipf.namespace, ipf.name)
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		podName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		ensureIpfailoverMasterBackup(oc, ipf.namespace, podName)

		g.By("add 254 VIPs for the failover group")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `OPENSHIFT_HA_VIRTUAL_IPS=192.168.254.1-254`)

		g.By("Exclude VIP '9' from the ipfailover group")
		getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		setEnvVariable(oc, ipf.namespace, "deploy/"+ipf.name, `HA_EXCLUDED_VRRP_IDS=9`)

		g.By("verify from the ipfailover pod, the excluded VRRP_ID is configured")
		ensurePodWithLabelReady(oc, ipf.namespace, "ipfailover=hello-openshift")
		newPodName := getPodListByLabel(oc, ipf.namespace, "ipfailover=hello-openshift")
		checkenv := pollReadPodData(oc, ipf.namespace, newPodName[0], "/usr/bin/env ", "HA_EXCLUDED_VRRP_IDS")
		o.Expect(checkenv).To(o.ContainSubstring("HA_EXCLUDED_VRRP_IDS=9"))

		g.By("verify the excluded VIP is removed from the router_ids of ipfailover pods")
		routerIds := pollReadPodData(oc, ipf.namespace, newPodName[0], `cat /etc/keepalived/keepalived.conf`, `virtual_router_id`)
		o.Expect(routerIds).NotTo(o.ContainSubstring(`virtual_router_id 9`))
	})
})
