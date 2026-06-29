package router
import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type ipfailoverDescription struct {
	name        string
	namespace   string
	image       string
	vip         string
	HAInterface string
	template    string
}

func exactNodeDetails(oc *exutil.CLI) (int, string) {
	linuxWorkerDetails, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=,kubernetes.io/os=linux").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeCount := int(strings.Count(linuxWorkerDetails, "Ready")) - (int(strings.Count(linuxWorkerDetails, "SchedulingDisabled")) + int(strings.Count(linuxWorkerDetails, "NotReady")))
	e2e.Logf("Linux worker node details are:\n%v", linuxWorkerDetails)
	e2e.Logf("Available linux worker node count is: %v", nodeCount)
	// checking other type workers for debugging
	nonLinuxWorker, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=,kubernetes.io/os!=linux").Output()
	if !strings.Contains(nonLinuxWorker, "No resources found") {
		e2e.Logf("Found non linux worker nodes and details are:\n%v", nonLinuxWorker)
	}
	remoteWorker, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node.openshift.io/remote-worker").Output()
	if !strings.Contains(remoteWorker, "No resources found") {
		e2e.Logf("Found remote worker nodes and details are:\n%v", remoteWorker)
	}
	outpostWorker, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=topology.ebs.csi.aws.com/outpost-id").Output()
	if !strings.Contains(outpostWorker, "No resources found") {
		e2e.Logf("Found outpost worker nodes and details are:\n%v", outpostWorker)
	}
	localZoneWorker, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/edge").Output()
	if !strings.Contains(localZoneWorker, "No resources found") {
		e2e.Logf("Found local zone worker nodes and details are:\n%v", localZoneWorker)
	}
	return nodeCount, linuxWorkerDetails
}

func ensurePodWithLabelReady(oc *exutil.CLI, ns, label string) {
	err := waitForPodWithLabelReady(oc, ns, label)
	// print pod status and logs for debugging purpose if err
	if err != nil {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label).Output()
		e2e.Logf("All pods with label %v are:\n%v", label, output)
		logs, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", ns, "-l", label, "--tail=10").Output()
		e2e.Logf("The logs of all labeled pods are:\n%v", logs)
	}
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("max time reached but the pods with label %v are not ready", label))
}

func setEnvVariable(oc *exutil.CLI, ns, resource, envstring string) {
	err := oc.AsAdmin().WithoutNamespace().Run("set").Args("env", "-n", ns, resource, envstring).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	time.Sleep(10 * time.Second)
}

func describePodResource(oc *exutil.CLI, podName, namespace string) string {
	podDescribe, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", podName, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return podDescribe
}

func getImagePullSpecFromPayload(oc *exutil.CLI, image string) string {
	var pullspec string
	baseDir := "e2e/testdata/router"
	indexTmpPath := filepath.Join(baseDir, getRandomString())
	dockerconfigjsonpath := filepath.Join(indexTmpPath, ".dockerconfigjson")
	defer exec.Command("rm", "-rf", indexTmpPath).Output()
	err := os.MkdirAll(indexTmpPath, 0755)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/pull-secret", "-n", "openshift-config", "--confirm", "--to="+indexTmpPath).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	pullspec, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("release", "info", "--image-for="+image, "-a", dockerconfigjsonpath).Output()
	if err != nil {
		g.Skip("Skipping as failed to get image pull spec from the payload")
	}
	e2e.Logf("the pull spec of image %v is: %v", image, pullspec)
	return pullspec
}

func ensureIpfailoverMasterBackup(oc *exutil.CLI, ns string, podList []string) (string, string) {
	var masterPod, backupPod string
	// The sleep is given for the election process to finish
	time.Sleep(10 * time.Second)
	waitErr := wait.Poll(3*time.Second, 90*time.Second, func() (bool, error) {
		podLogs1, err1 := compat_otp.GetSpecificPodLogs(oc, ns, "", podList[0], "Entering")
		if err1 != nil {
			// Pod logs might not be ready yet or grep found no matches, retry
			return false, nil
		}
		logList1 := strings.Split((strings.TrimSpace(podLogs1)), "\n")
		e2e.Logf("The first pod log's failover status:- %v", podLogs1)
		podLogs2, err2 := compat_otp.GetSpecificPodLogs(oc, ns, "", podList[1], "Entering")
		if err2 != nil {
			// Pod logs might not be ready yet or grep found no matches, retry
			return false, nil
		}
		logList2 := strings.Split((strings.TrimSpace(podLogs2)), "\n")
		e2e.Logf("The second pod log's failover status:- %v", podLogs2)

		// Checking whether the first pod is failover state master and second pod backup
		if strings.Contains(logList1[len(logList1)-1], "(ipfailover_VIP_1) Entering MASTER STATE") {
			if strings.Contains(logList2[len(logList2)-1], "(ipfailover_VIP_1) Entering BACKUP STATE") {
				masterPod = podList[0]
				backupPod = podList[1]
				return true, nil
			}
			// Checking whether the second pod is failover state master and first pod backup
		} else if strings.Contains(logList1[len(logList1)-1], "(ipfailover_VIP_1) Entering BACKUP STATE") {
			if strings.Contains(logList2[len(logList2)-1], "(ipfailover_VIP_1) Entering MASTER STATE") {
				masterPod = podList[1]
				backupPod = podList[0]
				return true, nil
			}
		}
		e2e.Logf("The ipfailover seems not yet converged, retrying again...")
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("Reached max time allowed but IPfailover seems not working as expected."))
	e2e.Logf("The Master pod is %v and Backup pod is %v", masterPod, backupPod)
	return masterPod, backupPod
}

func replaceIPOctet(ipaddress []string, octet int, octetValue string) string {
	ipv4address := ipaddress[0]
	if strings.Count(ipaddress[0], ":") >= 2 {
		ipv4address = ipaddress[1]
	}
	ipList := strings.Split(ipv4address, ".")
	ipList[octet] = octetValue
	vip := strings.Join(ipList, ".")
	e2e.Logf("The modified ipaddress is %s ", vip)
	return vip
}

func getPodListByLabel(oc *exutil.CLI, namespace string, label string) []string {
	var podList []string
	podNameAll, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, "pod", "-l", label, "-ojsonpath={.items..metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podList = strings.Split(podNameAll, " ")
	e2e.Logf("The pod list is %v", podList)
	return podList
}

func getVipOwnerPod(oc *exutil.CLI, ns string, podname []string, vip string) string {
	cmd := fmt.Sprintf("ip address |grep %s", vip)
	var primaryNode string
	for i := 0; i < len(podname); i++ {
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", ns, podname[i], "--", "bash", "-c", cmd).Output()
		if len(podname) == 1 && output == "command terminated with exit code 1" {
			e2e.Failf("The given pod is not master")
		}
		if output == "command terminated with exit code 1" {
			e2e.Logf("This Pod %v does not have the VIP", podname[i])
		} else if strings.Contains(output, vip) {
			e2e.Logf("The pod owning the VIP is %v", podname[i])
			primaryNode = podname[i]
			break
		} else {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
	return primaryNode
}

func slicingElement(element string, podList []string) []string {
	var newPodList []string
	for index, pod := range podList {
		if pod == element {
			newPodList = append(podList[:index], podList[index+1:]...)
			break
		}
	}
	e2e.Logf("The remaining pod/s in the list is %v", newPodList)
	return newPodList
}

func waitForPrimaryPod(oc *exutil.CLI, ns string, pod string, vip string) {
	cmd := fmt.Sprintf("ip address |grep %s", vip)
	waitErr := wait.Poll(5*time.Second, 50*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", ns, pod, "--", "bash", "-c", cmd).Output()
		primary := false
		if strings.Contains(output, vip) {
			e2e.Logf("The new pod %v is the master", pod)
			primary = true
		} else {
			e2e.Logf("pod failed to become master yet, retrying...the error is %v", output)
		}
		return primary, nil
	})
	// for debugging
	output1, _ := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", ns, pod, "--", "bash", "-c", "ip address").Output()
	compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("max time reached, pod failed to become master and the entire ip details of the pod is %v", output1))
}

func pollReadPodData(oc *exutil.CLI, ns, routername, executeCmd, searchString string) string {
	cmd := fmt.Sprintf("%s | grep \"%s\"", executeCmd, searchString)
	var output string
	var err error
	waitErr := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		output, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", ns, routername, "--", "bash", "-c", cmd).Output()
		if err != nil {
			e2e.Logf("failed to get search string: %v, retrying...", err)
			return false, nil
		}
		return true, nil
	})
	e2e.Logf("the matching part is: %s", output)
	compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("reached max time allowed but cannot find the search string."))
	return output
}

func addPrivilegedLabelToNamespace(oc *exutil.CLI, ns string) {
	enforceLabel := "pod-security.kubernetes.io/enforce=privileged"
	auditLabel := "pod-security.kubernetes.io/audit=privileged"
	warnLabel := "pod-security.kubernetes.io/warn=privileged"
	err := oc.AsAdmin().WithoutNamespace().Run("label").Args("namespace", ns, enforceLabel, auditLabel, warnLabel, "--overwrite").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Successfully added privileged labels (enforce, audit, warn) to namespace %s", ns)
}

func unicastIPFailover(oc *exutil.CLI, ns, failoverName string) {
	platformtype := compat_otp.CheckPlatform(oc)

	if platformtype == "nutanix" || platformtype == "none" {
		getPodListByLabel(oc, oc.Namespace(), "ipfailover=hello-openshift")
		workerIPAddress, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-ojsonpath={.items[*].status.addresses[0].address}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		modifiedIPList := strings.Split(workerIPAddress, " ")
		if len(modifiedIPList) < 2 {
			e2e.Failf("There is not enough IP addresses to add as unicast peer")
		}
		ipList := strings.Join(modifiedIPList, ",")
		cmd := fmt.Sprintf("OPENSHIFT_HA_UNICAST_PEERS=%v", ipList)
		setEnvVariable(oc, ns, "deploy/"+failoverName, "OPENSHIFT_HA_USE_UNICAST=true")
		setEnvVariable(oc, ns, "deploy/"+failoverName, cmd)
	}
}

func getPodIP(oc *exutil.CLI, namespace string, podName string) []string {
	ipStack := checkIPStackType(oc)
	var podIp []string
	if (ipStack == "ipv6single") || (ipStack == "ipv4single") {
		podIp1, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[0].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod  %s IP in namespace %s is %q", podName, namespace, podIp1)
		podIp = append(podIp, podIp1)
	} else if ipStack == "dualstack" {
		podIp1, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[0].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod's %s 1st IP in namespace %s is %q", podName, namespace, podIp1)
		podIp2, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.podIPs[1].ip}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The pod's %s 2nd IP in namespace %s is %q", podName, namespace, podIp2)
		podIp = append(podIp, podIp1, podIp2)
	}
	return podIp
}

func (ipf *ipfailoverDescription) create(oc *exutil.CLI, ns string) {
	// Add Pod Security admission labels to allow privileged pods
	addPrivilegedLabelToNamespace(oc, ns)
	// create ServiceAccount and add it to related SCC
	_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("sa", "ipfailover", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "privileged", "-z", "ipfailover", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	// create the ipfailover deployment
	err = createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ipf.template, "-p", "NAME="+ipf.name, "NAMESPACE="+ipf.namespace, "IMAGE="+ipf.image, "HAINTERFACE="+ipf.HAInterface)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	jsonCfg := parseToJSON(oc, parameters)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Execute()
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func parseToJSON(oc *exutil.CLI, parameters []string) string {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "-temp-resource.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))
	e2e.Logf("the file of resource is %s", jsonCfg)
	return jsonCfg
}

func waitForPodWithLabelReady(oc *exutil.CLI, ns, label string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label, `-ojsonpath={.items[*].status.conditions[?(@.type=="Ready")].status}`).Output()
		e2e.Logf("the Ready status of pod is %v", status)
		if err != nil || status == "" {
			e2e.Logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "False") {
			e2e.Logf("the pod Ready status not met; wanted True but got %v, retrying...", status)
			return false, nil
		}
		return true, nil
	})
}

func readPodData(oc *exutil.CLI, podname string, ns string, executeCmd string, searchString string) string {
	cmd := fmt.Sprintf("%s | grep \"%s\"", executeCmd, searchString)
	output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", ns, podname, "--", "bash", "-c", cmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the matching part is: %s", output)
	return output
}


func checkIPStackType(oc *exutil.CLI) string {
	svcNetwork, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.serviceNetwork}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Count(svcNetwork, ":") >= 2 && strings.Count(svcNetwork, ".") >= 2 {
		return "dualstack"
	} else if strings.Count(svcNetwork, ":") >= 2 {
		return "ipv6single"
	} else if strings.Count(svcNetwork, ".") >= 2 {
		return "ipv4single"
	}
	return ""
}
