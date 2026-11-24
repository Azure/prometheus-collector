# Customer settings variants

Each folder is one settings variant containing its customer ConfigMaps and any
monitor custom resources. `catalog.json` holds the variant metadata, including
the Ginkgo labels and expected outcome; `coverage.json` maps each settings key
to the variants that exercise it. Scenario rows in
[`../matrix.json`](../matrix.json) choose which variants run on which cluster.
Fixture paths in the catalog are relative to the matching variant folder.

## How a variant runs

The scenario runner saves the ama-metrics ConfigMaps listed in `catalog.json`,
applies the variant, waits for the collector to pick it up, runs the variant's
labeled `configprocessing` specs, then restores the original ConfigMaps. Variants
on the same cluster run one at a time.

Add any cluster-wide ConfigMap a variant may reset to `lifecycle.resetConfigMaps`;
`lifecycle.settingsConfigMap` identifies the settings map that needs special
resource-version handling.

Transition variants (`customer-transitions-*`, `secret-transitions-*`) start from
the applied variant and have their Ginkgo spec apply further changes in place, for
example changing namespace filters or rotating basic-auth credentials, then check
that fresh samples reflect each change.

## Add a variant

1. Copy the closest folder and change only the settings under test.
2. Add its metadata to `catalog.json` and map its keys in `coverage.json`.
3. Add the variant to a scenario row in `matrix.json`.