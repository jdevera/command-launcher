# Testing

The project uses separate test layers because they provide different evidence.

## Unit tests

Run the unit suite with:

```shell
go test ./...
```

This is the default, cacheable Go test target. It does not build or execute the
CLI black-box tests.

## CLI black-box tests

Run the self-contained CLI scenarios with:

```shell
go test -tags=e2e -count=1 ./test/e2e
```

These tests build the launcher, execute it as a subprocess, and isolate its
home directory. The remote-package lifecycle scenario uses a temporary HTTP
server so it can test installation and updates without network access. It does
not claim to test public HTTPS.

`-count=1` is required because the e2e package builds production code as a
subprocess. Go's test-result cache cannot otherwise detect production-source
changes as test inputs.

## Remote HTTPS smoke test

Run the real-network smoke test with:

```shell
go test -tags=e2e,remote -count=1 -run '^TestRemoteHTTPSRegistry$' ./test/e2e
```

This test always contacts a non-loopback HTTPS registry. It rejects plain HTTP,
localhost, literal loopback addresses, and filesystem URLs. It proves that the
compiled launcher can use the host operating system's trusted certificate roots
to download, install, and execute a package.

The remote is declared once in [`remote-fixture.json`](remote-fixture.json).
The base URL is resolved in this order:

1. `TEST_REMOTE_BASE_URL`, when explicitly supplied.
2. `GITHUB_REPOSITORY` and `GITHUB_SERVER_URL` in GitHub Actions.
3. The local Git `origin` remote.
4. `default_repository` from `remote-fixture.json` when `.git` is unavailable.

The configured tag and registry path are always read from
`remote-fixture.json`. The current fallback repository is
`jdevera/command-launcher`, allowing the test to run from source archives that
do not contain `.git` metadata.

The test requires network access and a published fixture tag. A failure to
resolve or reach the configured remote is a test failure, not a skip.

### Publishing updated remote fixtures

Treat fixture tags as immutable:

1. Change and commit the registry files.
2. Push the commit that contains those files.
3. Create and push a new tag such as `remote-fixtures-v2` on that commit.
4. Change the `tag` field in `remote-fixture.json` to the new tag.
5. Commit the pointer update.

Do not move an existing fixture tag. Unique tags keep previous test runs
reproducible and avoid stale raw-content caches.

Release-tag workflows run this smoke test once on Linux, Windows, and macOS.
The `amd64` matrix entry is used to avoid repeating architecture-independent
TLS coverage.

## Shell integration tests

Run all legacy shell integration tests with:

```shell
./test/integration.sh
```

Run selected scripts by passing their names without `.sh`:

```shell
./test/integration.sh test-basic test-exit-code
```

These suites require Bash and are being migrated incrementally to Go
black-box tests. Their external registry references use the same
`TEST_REMOTE_BASE_URL` resolution described above. Tests that need a second
registry or a Git repository create those fixtures from checked-in files, so
they do not depend on another GitHub repository.
