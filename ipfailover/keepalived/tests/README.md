# ipfailover OTE Test Extension

All 6 ipfailover tests have been successfully migrated from openshift-tests-private to the OTE (openshift-tests-extension) framework.

## Test Suites

### ipfailover/all
All 6 tests (must run with `--max-concurrency=1`)

### ipfailover/conformance/serial
4 serial tests (must run sequentially):
- 41025 - support to deploy ipfailover
- 41028 - ipfailover configuration can be customized by ENV
- 41029 - ipfailover can support up to 255 VIPs
- 49214 - Excluding existing VRRP cluster ID

### ipfailover/disruptive
2 disruptive tests:
- 41027 - pod and service automatically switched over
- 41030 - preemption strategy for keepalived ipfailover

## Where These Tests Run

The extension declares environment selectors, so `openshift-tests` only schedules
these tests on:

- platform `baremetal`, `none`, `vsphere`, `openstack` or `nutanix`
- a cluster that is not single-stack IPv6 (dual stack is fine)
- a topology that is neither `External` (HyperShift guest) nor `SingleReplica`
  (single-node, and MicroShift, which reports as platform `none`)

They also need at least two worker nodes, which no selector can express, so
`BeforeEach` checks for that at runtime.

The selectors only apply when environment flags are passed. Running the binary
directly without them, as the commands below do, schedules all six tests whatever
the cluster -- there, the runtime checks in `BeforeEach` are the only thing that
skips them.

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
./bin/ipfailover-tests-ext run-test "[OTP][sig-network-edge] Network_Edge Critical-41025-support to deploy ipfailover [Serial]"

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
