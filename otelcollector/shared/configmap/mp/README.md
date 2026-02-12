# Managed Prometheus Configmap Tests

This package contains the Ginkgo-based integration tests that guard the Managed Prometheus (MP) configmap parser. The tests simulate every combination of platform (Linux ReplicaSet, Linux DaemonSet, Windows DaemonSet) and configmap settings to ensure that:

1. The configmapparser writes the correct environment variables for the agent pods.
2. The generated Prometheus configuration matches the expected fixtures under `./testdata`.

## Repository layout

- `configmapparser_test.go` – master spec that drives all scenarios via helper contexts.
- `testdata/` – canonical Prometheus config fixtures that every scenario compares against.
- `configmap-test-cases/` – TOML snippets used by some contexts to mimic multi-section configmaps (both v1 and v2 schemas).
- `../common/testhelpers` – shared helpers that provides temp file helpers, env overrides, and comparison utilities.

## Running the tests

```bash
cd otelcollector/shared/configmap/mp
go test .
```


## How the specs work

Each context builds a scenario by writing synthetic configmap files into `/tmp/settings`, seeding env vars via `testhelpers.SetManagedPrometheusEnvVars`, and then calling the configmapparser's main function `processConfigFiles()`.

The common `checkResults(...)` helper performs three validations:

1. **Env vars** – compares actual vars against `expectedEnvVars` using `testhelpers.CheckEnvVars`.
2. **Keep-list and scrape-interval hashes** – parses the generated YAML files and diff-checks them against maps built by `testhelpers.ExpectedKeepListMap` / `ExpectedScrapeIntervalMap`.
3. **Prometheus configs** – reads `mergedDefaultConfigPath` (and optionally the merged custom config) and asserts equivalence against the YAML in `./testdata`.

All scrape configs are sorted by `job_name` prior to comparison so fixture ordering does not depend on the ordering that the configmapparser produces.

## Test coverage

The suite currently exercises the following scenarios (each runs across Linux ReplicaSet, Linux DaemonSet, and Windows DaemonSet):

- When the settings configmap does not exist
- When the settings configmap sections exist but are empty
- When the settings configmap sections exist and are not default
- When some of the configmap sections exist but not all
- When the configmap sections exist but all scrape jobs are false
- When minimal ingestion is false and has keeplist regex values
- When minimal ingestion is false and has no keeplist regex values
- When the custom configmap exists
    - And the settings configmap sections do not exist
    - And the settings configmap sections have all default scrape configs set to false
- When the settings configmap uses v2 and the sections exist but are empty
- When the settings configmap uses v2 and the sections are not default
- When the settings configmap uses v2, minimal ingestion is false, and not all sections are present


## Adding or updating scenarios

Use the following checklist whenever you add a new scenario or update an existing one:

1. **Decide the context** – pick the closest existing `Context(...)` block in `configmapparser_test.go` or add a new one if the behavior differs materially (new schema version, new feature gate, etc.).
2. **Author temp config files** – leverage `testhelpers.MustCreateTempFile` to write the configmap sections (default scrape settings, keep-list, scrape intervals, pod annotation regex, etc.). Remember that anything omitted defaults to empty.
3. **Set env expectations** – start with `testhelpers.DefaultManagedPrometheusEnvVars()` and override only what the scenario mutates. If the scenario toggles scrape jobs, use `testhelpers.BuildEnvVarOverrides` so both tests and parser stay in sync.
4. **Capture expected Prometheus output** – run the parser once (e.g., `go test -run '<context regexp>' .`), grab the generated content from the output, and copy it into `testdata/<descriptive-name>.yaml`.
5. **Update `configmap-test-cases` if needed** – some contexts drive the parser by pointing at TOML fixtures. If your scenario depends on a new setting, add it to both v1 and v2 directories so upgrade tests stay aligned.
6. **Add assertions** – point your new `It(...)` block at `checkResults(...)`, passing the expected env/keep-list/scrape interval maps and the paths of any fixtures you created.
7. **Run the suite** – execute `go test .` (or a focused subset) until everything passes.
