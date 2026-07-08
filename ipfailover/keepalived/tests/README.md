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

## How to Run

```bash
export KUBECONFIG=/path/to/kubeconfig

# Build the binary
make build

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

```
tests/
├── cmd/main.go              # OTE entry point, suite definitions
├── e2e/
│   ├── ipfailover.go       # 6 test implementations
│   ├── util.go            # Helper functions
│   └── testdata/
│       └── router/         # Test YAML fixtures (used directly, no bindata)
├── Makefile               # Simple build (no bindata generation)
├── go.mod                 # Dependencies
└── README.md              # This file
```
