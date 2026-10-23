"""Check annotated reference-app metrics in Container Insights, not AMW."""

import argparse
from datetime import datetime, timezone
import json
import os
import shutil
import subprocess
import time


METRICS = {"myapp_temperature", "myapp_measurements_total"}


def require_metrics(rows, pods):
    observed = {(r["Pod"], r["ScrapeUrl"], r["Name"]) for r in rows if int(r["Rows"]) > 0}
    expected = {(p["metadata"]["name"], f"http://{p['status']['podIP']}:2112/metrics", m)
                for p in pods for m in METRICS}
    missing = expected - observed
    if not expected or missing:
        raise AssertionError(f"Missing fresh reference-app metrics: {sorted(missing)}")


def self_test():
    pods = [{"metadata": {"name": "new-pod"}, "status": {"podIP": "10.0.0.1"}}]
    rows = [{"Pod": "new-pod", "ScrapeUrl": "http://10.0.0.1:2112/metrics",
             "Name": name, "Rows": "1"} for name in METRICS]
    require_metrics(rows, pods)
    for broken in ([], rows[:1], [dict(r, Pod="old-pod") for r in rows],
                   [dict(r, ScrapeUrl="http://wrong-route/metrics") for r in rows],
                   [dict(r, Rows="0") for r in rows]):
        try:
            require_metrics(broken, pods)
        except AssertionError:
            continue
        raise AssertionError("Negative control incorrectly passed")
    print("Missing-family, stale-pod, wrong-route and zero-row negative controls rejected")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true")
    for name in ("subscription", "workspace", "cluster-resource-id", "kubeconfig", "context", "since"):
        parser.add_argument("--" + name)
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not all((args.subscription, args.workspace, args.cluster_resource_id,
                args.kubeconfig, args.context, args.since)):
        parser.error("subscription, workspace, cluster-resource-id, kubeconfig, context and since are required")
    prefix = f"/subscriptions/{args.subscription}/"
    if not args.cluster_resource_id.lower().startswith(prefix.lower()) or "'" in args.cluster_resource_id:
        parser.error("cluster-resource-id must belong to the explicit subscription")
    since = datetime.fromisoformat(args.since.replace("Z", "+00:00"))
    if since.tzinfo is None:
        parser.error("since must include a timezone")
    since_text = since.astimezone(timezone.utc).isoformat()
    os.environ["AZURE_SUBSCRIPTION_ID"] = args.subscription
    kubectl = shutil.which("kubectl")
    az = shutil.which("az")
    if not kubectl or not az:
        raise RuntimeError("kubectl and Azure CLI must already be installed and authenticated")

    def run(command):
        result = subprocess.run(command, capture_output=True, text=True,
                                timeout=min(60, max(1, deadline - time.monotonic())))
        if result.returncode:
            raise RuntimeError(f"Command failed ({result.returncode}): {result.stderr or result.stdout}")
        return json.loads(result.stdout)

    base = [kubectl, "--kubeconfig", args.kubeconfig, "--context", args.context, "--request-timeout=25s"]
    deadline = time.monotonic() + 300
    while True:
        pods = run(base + ["get", "pods", "-n", "default", "-l",
                          "app=container-insights-reference-app", "-o", "json"])["items"]
        pods = [p for p in pods if not p["metadata"].get("deletionTimestamp")]
        ready = pods and all(p["status"].get("podIP") and
                             p["status"].get("containerStatuses") and
                             all(c["ready"] for c in p["status"]["containerStatuses"]) for p in pods)
        query = (
            f"InsightsMetrics | where TimeGenerated >= datetime({since_text}) "
            "| where Namespace == 'prometheus' and Origin == 'container.azm.ms/telegraf' "
            "| where Name in ('myapp_temperature', 'myapp_measurements_total') "
            "| extend T=todynamic(Tags) "
            "| where tostring(T.app) == 'container-insights-reference-app' and tostring(T.pod_namespace) == 'default' "
            f"| where tostring(T['container.azm.ms/clusterId']) =~ '{args.cluster_resource_id}' "
            "| summarize Rows=count() by Name, Pod=tostring(T.pod_name), ScrapeUrl=tostring(T.scrapeUrl)"
        )
        rows = run([az, "monitor", "log-analytics", "query", "--subscription", args.subscription,
                    "--workspace", args.workspace, "--analytics-query", query, "-o", "json", "--only-show-errors"])
        if ready:
            try:
                require_metrics(rows, pods)
            except AssertionError as error:
                print(error, flush=True)
            else:
                print(json.dumps(rows, indent=2))
                return
        else:
            print("Waiting for Ready reference-app pods", flush=True)
        if time.monotonic() >= deadline:
            raise TimeoutError("Reference-app readiness or fresh InsightsMetrics ingestion not proven within five minutes")
        time.sleep(min(15, max(0, deadline - time.monotonic())))


if __name__ == "__main__":
    main()
