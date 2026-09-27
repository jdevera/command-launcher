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
home directory. They cover basic discovery and help, configuration output,
external-command exit codes, executable-derived runtime identity, YAML
manifests, and the remote-package lifecycle. The remote-package scenario uses
a temporary HTTP server so it can test installation and updates without
network access. It does not claim to test public HTTPS.

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

Set `COMMAND_LAUNCHER_E2E_BINARY` to exercise an existing launcher binary
instead of having the test harness build one. Release-tag workflows use this to
smoke-test the exact amd64 artifact produced by `build.sh` on Linux, Windows,
and macOS before packaging and publication.

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
The `amd64` matrix entry provides an executable artifact on the current hosted
runners and avoids repeating architecture-independent TLS coverage. ARM64
artifacts are built and packaged but are not yet runtime-smoked on native
ARM64 runners.

Matrix jobs only upload workflow artifacts. Packaging waits for every Linux,
Windows, and macOS build and test job to succeed, and a single final job then
publishes all binaries, the combined archive, and `latest.yaml` together. If
any matrix entry fails, no job with GitHub release write permission runs.

## Shell integration tests

Run all legacy shell integration tests with:

```shell
./test/integration.sh
```

Run selected scripts by passing their names without `.sh`:

```shell
./test/integration.sh test-cmd-context test-flag-arg
```

The remaining 12 suites require Bash and are being migrated incrementally to
Go black-box tests. They use the checked-in `examples/remote-repo` directory as
their default registry, exposed through a `file://` URL. Tests that need a
second registry or a Git repository also create those fixtures from checked-in
files, so the normal integration run does not require network access.

Set `TEST_REMOTE_BASE_URL` explicitly to run the legacy scenarios against a
different registry. This does not affect the release-only HTTPS smoke test.
