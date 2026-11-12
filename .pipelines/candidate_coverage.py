"""List planned candidate rows and explicit gaps; this is not execution evidence."""

import argparse
import json
from pathlib import Path


SCENARIOS = ("default", "classic-azure-cni", "acns", "otlp")
SECRET_WATCH_BLOCKER = "Secret-watch enforcement requires an available Kubernetes >=1.37 baseline; current approved baseline is 1.36.5."


def selected_variants(root, row, catalog):
    if row["settingsVariants"] != ["*"]:
        return row["settingsVariants"]
    selected = []
    for entry in catalog["variants"]:
        manifest = json.loads((root / "settings" / entry["manifest"]).read_text())
        features = set(row["clusterFeatures"])
        if (set(manifest["requires"]["clusterFeatures"]) <= features
                and set(manifest["platforms"]) <= set(row["platforms"])
                and not manifest.get("helmValues") and not manifest.get("addonParams")):
            selected.append(entry["id"])
    return selected


def plan(root, scenario):
    matrix = json.loads((root / "matrix.json").read_text())
    catalog = json.loads((root / "settings" / "catalog.json").read_text())
    rows = {row["id"]: row for row in matrix["scenarios"]}
    runnable = {name for name, row in rows.items()
                if not row.get("blockedReason") and name != "operator-secret-watch"}
    if runnable != set(SCENARIOS):
        raise ValueError("Candidate selector does not account for every runnable matrix row")
    if scenario not in (*SCENARIOS, "acns-cilium"):
        raise ValueError(f"Unsupported candidate scenario: {scenario}")
    blocked = {name: row["blockedReason"] for name, row in rows.items() if row.get("blockedReason")}
    blocked["operator-secret-watch"] = SECRET_WATCH_BLOCKER
    covered = {variant for name in SCENARIOS
               for variant in selected_variants(root, rows[name], catalog)}
    variants = []
    for variant in catalog["variants"]:
        name = variant["id"]
        if name in covered:
            continue
        reasons = [reason for row_id, reason in blocked.items() if name in rows[row_id]["settingsVariants"]]
        if not reasons:
            raise ValueError(f"Unaccounted settings variant: {name}")
        variants.append({"id": name, "status": "BLOCKED", "reason": "; ".join(reasons)})
    return {
        "status": "PLANNED_WITH_BLOCKERS", "executed": False, "selectedScenario": scenario,
        "applicableScenarios": list(SCENARIOS),
        "blockedScenarios": [{"id": name, "status": "BLOCKED", "reason": reason}
                             for name, reason in blocked.items()],
        "blockedVariants": variants,
        "blockedTestSuites": [
            {
                "id": "arc-conformance", "status": "BLOCKED",
                "reason": "Arc conformance uses a separate Arc/Sonobuoy environment and is not run by the AKS candidate adapter.",
            },
            {
                "id": "ccp-health", "status": "BLOCKED",
                "reason": "CCP health requires its standalone-underlay adapter; ordinary AKS is not CCP coverage.",
            },
            {
                "id": "regionTests", "status": "BLOCKED",
                "reason": "Missing owned recording-rule and AMW usage/limits authorization adapter; "
                          "requires parmRuleName and parmAmwResourceId. Health suites are not a substitute.",
            },
        ],
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--scenario", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--require-unblocked", action="store_true")
    args = parser.parse_args()
    result = plan(args.root, args.scenario)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result))
    blockers = (len(result["blockedScenarios"]), len(result["blockedVariants"]),
                len(result["blockedTestSuites"]))
    if args.require_unblocked and any(blockers):
        raise SystemExit(
            "Candidate coverage gate blocked: "
            f"{blockers[0]} scenarios, {blockers[1]} settings variants, "
            f"{blockers[2]} test suites remain blocked"
        )
