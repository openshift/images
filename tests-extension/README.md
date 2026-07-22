# IPFailover Tests Extension

This directory provides the standard `tests-extension/` structure expected by OpenShift CI for running ipfailover OTE (OpenShift Tests Extension) tests.

## Directory Structure

The actual test implementation lives in `ipfailover/keepalived/tests/`. This directory acts as a wrapper that provides the standard interface expected by CI.

## Usage

### Build the test binary
```bash
make build
```

This will:
1. Build the test extension in `ipfailover/keepalived/tests/`
2. Copy the binary to `tests-extension/bin/ipfailover-tests-ext`

### Run tests
```bash
./bin/ipfailover-tests-ext run-suite ipfailover/all --max-concurrency=1
```

### Verify structure
```bash
make verify
make verify-metadata
```

## CI Integration

This structure is consumed by the OpenShift CI jobs defined in the `openshift/release` repository. The CI jobs:

1. Clone the `openshift/images` repo
2. Change to `tests-extension/`
3. Run `make build`
4. Execute `./bin/ipfailover-tests-ext run-suite ipfailover/all --max-concurrency=1`

## See Also

- Actual test implementation: `../ipfailover/keepalived/tests/`
- Test documentation: `../ipfailover/keepalived/tests/README.md`
