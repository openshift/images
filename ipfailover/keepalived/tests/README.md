# ipfailover OTE Test Extension

All 6 ipfailover tests have been successfully migrated from openshift-tests-private to the OTE (openshift-tests-extension) framework.

## Naming

Case IDs are Ginkgo labels (`g.Label("OCP-NNNNN")`), not part of the test name.
Authors are recorded as a source comment immediately above each `g.It`:

```go
//Author:mjoseph
g.It("ipfailover configuration can be customized by ENV [Serial]", g.Label("OCP-41028"), ote.Informing(), func() {
```

### Ownership

OTE requires each test to declare an owning component, either as a
`[Jira:Component]` tag in the test name or as a `ci-test-mapping` match. These
tests carry `[Feature:Router]` on the shared `Describe`, which combined with
`[sig-network-edge]` matches the existing `Networking / router` SIG matcher in
[ci-test-mapping](https://github.com/openshift-eng/ci-test-mapping). No
`ci-test-mapping` change is required for that matcher to apply; it is
pre-existing. Note that the `Suite:"Network_Edge"` matcher in the same
component does *not* apply here, since OTE-run tests report JUnit suite
`openshift-tests` uniformly rather than the private runner's suite name.

## Test Suites

### ipfailover/all
All 6 tests (must run with `--max-concurrency=1`)

### ipfailover/conformance/serial
6 serial tests (must run sequentially), including the two also tagged
`[Disruptive]`:
- OCP-41025 - support to deploy ipfailover
- OCP-41027 - pod and service automatically switched over [Disruptive]
- OCP-41028 - ipfailover configuration can be customized by ENV
- OCP-41029 - ipfailover can support up to 255 VIPs
- OCP-41030 - preemption strategy for keepalived ipfailover [Disruptive]
- OCP-49214 - Excluding existing VRRP cluster ID

### ipfailover/disruptive
Standalone suite with no conformance parent, for running the two disruptive
cases on their own:
- OCP-41027 - pod and service automatically switched over
- OCP-41030 - preemption strategy for keepalived ipfailover

## Where These Tests Run

The extension declares environment selectors, so `openshift-tests` only schedules
these tests on:

- platform `baremetal`, `none`, `vsphere`, `openstack` or `nutanix`
- the `config.openshift.io` API group enabled (absent on MicroShift)
- a cluster that is not single-stack IPv6 (dual stack is fine)
- a topology that is neither `External` (HyperShift guest) nor `SingleReplica`
  (single-node, and MicroShift, which reports as platform `none`)

They also need at least two worker nodes, which no selector can express, so
`BeforeEach` checks for that at runtime.

The selectors only apply when environment flags are passed. Running the binary
directly without them, as the commands below do, schedules all six tests whatever
the cluster. There, the runtime checks in `BeforeEach` are the only thing that
skips a genuinely unsupported platform or stack; a failed read of the
`infrastructure`/`network` cluster config fails the test rather than skipping
it, since by then the environment is assumed to support that API.

All six tests are registered as informing, so failures do not fail the job while
the migration beds in.

## How to Run

```bash
export KUBECONFIG=/path/to/kubeconfig

# Build the binary
make build

# List all test
./bin/ipfailover-tests-ext list ipfailover

# Run a single test
./bin/ipfailover-tests-ext run-test "[OTP][sig-network-edge][Feature:Router] Network_Edge support to deploy ipfailover [Serial]"

# Run all tests
./bin/ipfailover-tests-ext run-suite ipfailover/all --max-concurrency=1

# Run specific suites
./bin/ipfailover-tests-ext run-suite ipfailover/conformance/serial --max-concurrency=1
./bin/ipfailover-tests-ext run-suite ipfailover/disruptive --max-concurrency=1
```

**Important**: Tests must run serially (`--max-concurrency=1`) because they use `hostNetwork: true` and will conflict if run in parallel.

## Test Execution Time

- Single test: 1-5 minutes
- Serial suite: 10-15 minutes
- Disruptive suite: 7-10 minutes
- All tests: 15-20 minutes

## Files

```text
tests/
├── cmd/main.go              # OTE entry point, suite definitions
├── e2e/
│   ├── ipfailover.go       # 6 test implementations
│   ├── testdata_embed.go   # Embeds the fixture, materialized lazily at run time
│   ├── util.go            # Helper functions
│   └── testdata/
│       └── router/         # Test YAML fixtures (go:embed, no bindata)
├── Makefile               # Simple build (no bindata generation)
├── go.mod                 # Dependencies
└── README.md              # This file
```
