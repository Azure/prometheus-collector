# Helper script to generate the ARM template resources for the default Prometheus Alerts and Recording rules.
# This script outputs just the resources, you can copy / paste into a full ARM template
# The generated ARM resources are saved in output files: prometheus_alerts_arm.json, prometheus_rules_arm.json.

# Note: script assumes the presence of input files:
# - ./DefaultAlertsList.txt
# - ./DefaultRecordingRulesList.txt
# - ../mixins/kubernetes/prometheus_alerts.yaml
# - ../mixins/kubernetes/prometheus_rules.yaml

import json
import re
import yaml


class LiteralStr(str):
    """A subclass of str to represent literal strings."""

    pass


def setup_yaml_representer():
    """Setup YAML representer for LiteralStr to use literal style."""

    def change_style(style, representer):
        """Wrap the representer to change the scalar style."""

        def new_representer(dumper, data):
            scalar = representer(dumper, data)
            scalar.style = style
            return scalar

        return new_representer

    yaml.add_representer(
        LiteralStr, change_style("|", yaml.representer.SafeRepresenter.represent_str)
    )


def normalize_expression(rule):
    """Process and clean expression in Prometheus rules."""
    expr = rule["expr"].strip()
    if "\n" in expr:
        expr = LiteralStr(expr)
    # Space removal around parentheses and reduce multiple spaces to one
    expr = re.sub(r"\(\s+|\s+\)", lambda match: match.group(0).strip(), expr)
    expr = re.sub(r"\s+", " ", expr)
    rule["expr"] = expr


def load_file(file_path):
    """Helper function to load file content."""
    with open(file_path, "r") as file:
        return file.read().splitlines()


def load_yaml(file_path):
    """Helper function to load YAML content from a file."""
    with open(file_path, "r") as file:
        return yaml.safe_load(file)


def map_severity(label_value):
    """Map severity from label to corresponding integer value."""
    severity_map = {"critical": 0, "error": 1, "warning": 2, "info": 3, "verbose": 4}
    return severity_map.get(label_value, 3)  # Default to 3 (info) if not found


def convert_to_iso8601(duration):
    """Convert common duration format to ISO 8601 duration format."""
    match = re.match(r"(\d+)([smhdw])", duration)
    if not match:
        raise ValueError(f"Invalid duration format: {duration}")

    value, unit = match.groups()
    value = int(value)

    if unit == "s":
        return f"PT{value}S"
    elif unit == "m":
        return f"PT{value}M"
    elif unit == "h":
        return f"PT{value}H"
    elif unit == "d":
        return f"P{value}D"
    elif unit == "w":
        return f"P{value}W"


def process_rule(rule, include):
    """Process a single Prometheus rule and return the ARM alert or recording rule if applicable."""
    # Check if it's an alert rule and if it should be included
    if rule.get("alert") in include:
        labels = rule.get("labels", {})
        severity = map_severity(labels.get("severity", "info"))

        alert_for = rule.get("for")
        if alert_for:
            alert_for = convert_to_iso8601(alert_for)

        return {
            "type": "alert",
            "alert": rule["alert"],
            "expression": rule["expr"],
            "for": alert_for,
            "labels": labels,
            "severity": severity,
            "resolveConfiguration": {
                "autoResolved": True,
                "timeToResolve": "PT10M",
            },
            "actions": [{"actionGroupId": "[parameters('actionGroupResourceId')]"}],
        }
    # Check if it's a recording rule and if it should be included
    elif rule.get("record") in include:
        return {
            "type": "record",
            "record": rule["record"],
            "expression": rule["expr"],
            "labels": rule.get("labels", {}),
        }
    # Return None if the rule doesn't match the criteria for processing
    return None


def generate_arm_resources(include_file, rules_file, output_file):
    """Generate ARM template resources for the default alerts."""
    include = load_file(include_file)
    rules = load_yaml(rules_file)

    output = []
    for group in rules.get("groups", []):
        for rule in group.get("rules", []):
            normalize_expression(rule)
            arm_rule = process_rule(rule, include)
            if arm_rule:
                output.append(arm_rule)

    with open(output_file, "w") as file:
        json.dump(output, file, indent=2)


if __name__ == "__main__":
    # Setup YAML representer
    setup_yaml_representer()

    # Define input and output files
    prometheus_alerts = "../mixins/kubernetes/prometheus_alerts.yaml"
    prometheus_rules = "../mixins/kubernetes/prometheus_rules.yaml"
    include_alerts = "./DefaultAlertsList.txt"
    include_rules = "./DefaultRecordingRulesList.txt"
    arm_alerts = "./prometheus_alerts_arm.json"
    arm_rules = "./prometheus_rules_arm.json"

    # Generate ARM template resources
    generate_arm_resources(include_alerts, prometheus_alerts, arm_alerts)
    generate_arm_resources(include_rules, prometheus_rules, arm_rules)
