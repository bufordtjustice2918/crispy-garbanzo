#!/usr/bin/env python3
import argparse
import json
import subprocess
import xml.etree.ElementTree as ET
from pathlib import Path


ALLOWED_ROOTS = {"system", "interfaces", "firewall", "nat", "service", "policy"}
NODE_TAGS = {"node", "tagNode", "leafNode"}


def token_for_tag_name(name: str) -> str:
    n = name.lower()
    if n == "rule" or n.endswith("_rule") or n.endswith("-rule"):
        return "<id>"
    if n in {
        "ethernet",
        "bridge",
        "bonding",
        "dummy",
        "geneve",
        "input",
        "loopback",
        "macsec",
        "openvpn",
        "pppoe",
        "pseudo-ethernet",
        "tunnel",
        "virtual-ethernet",
        "vti",
        "vxlan",
        "wireguard",
    }:
        return "<ifname>"
    if n in {"asn", "table", "mark", "priority"}:
        return "<id>"
    return "<name>"


def infer_value_token(leaf: ET.Element) -> str:
    props = leaf.find("properties")
    if props is None:
        return "<value>"
    if props.find("valueless") is not None:
        return "enable"

    name = leaf.attrib.get("name", "").lower()
    if "address" in name or name in {"network", "prefix"}:
        return "<cidr>"
    if "port" in name:
        return "<port>"
    if name in {"asn", "id", "table", "mark", "priority"}:
        return "<id>"
    if "domain" in name or "host" in name:
        return "<name>"
    if "interface" in name:
        return "<ifname>"

    helps = props.findall("valueHelp")
    values = []
    for vh in helps:
        fmt = (vh.findtext("format") or "").strip()
        if fmt and len(fmt) < 32 and all(c.isalnum() or c in "-_" for c in fmt):
            values.append(fmt)
    if 1 < len(values) <= 8:
        return "<" + "|".join(values) + ">"
    if len(values) == 1:
        one = values[0].lower()
        if one in {"dhcp", "disable", "enable", "auto"}:
            return one

    return "<value>"


def leaf_description(leaf: ET.Element) -> str:
    help_text = (leaf.findtext("properties/help") or "").strip()
    if help_text:
        return help_text
    return f"Set {' '.join(leaf.attrib.get('name', '').split('-'))}"


def parse_definitions(transclude_script: Path, definition_file: Path) -> list[dict]:
    rendered = subprocess.check_output(
        [str(transclude_script), str(definition_file)],
        text=True,
    )
    root = ET.fromstring(rendered)
    out: list[dict] = []

    def walk(elem: ET.Element, path: list[str]) -> None:
        tag = elem.tag
        if tag not in NODE_TAGS:
            return

        name = elem.attrib.get("name")
        if not name:
            return

        if tag == "node":
            new_path = path + [name]
        elif tag == "tagNode":
            new_path = path + [name, token_for_tag_name(name)]
        else:
            value_token = infer_value_token(elem)
            props = elem.find("properties")
            multi = props is not None and props.find("multi") is not None
            out.append(
                {
                    "kind": "set",
                    "tokens": path + [name],
                    "value_token": value_token,
                    "description": leaf_description(elem),
                    "multi": multi,
                }
            )
            return

        children = elem.find("children")
        if children is None:
            return
        for child in children:
            walk(child, new_path)

    for child in root:
        if child.tag != "node":
            continue
        root_name = child.attrib.get("name", "")
        if root_name not in ALLOWED_ROOTS:
            continue
        walk(child, [])

    return out


def normalize_entry(e: dict) -> dict:
    out = {
        "kind": e["kind"],
        "tokens": [t.replace("-", "_") if t.startswith("<") and t.endswith(">") else t for t in e["tokens"]],
        "description": e.get("description", "Set value"),
    }
    if out["kind"] == "set":
        out["value_token"] = e.get("value_token", "<value>")
    if e.get("multi"):
        out["multi"] = True
    return out


def dedupe_sort(entries: list[dict]) -> list[dict]:
    seen = set()
    out = []
    for e in entries:
        key = (e["kind"], tuple(e["tokens"]), e.get("value_token", ""))
        if key in seen:
            continue
        seen.add(key)
        out.append(e)
    out.sort(key=lambda x: (x["kind"], " ".join(x["tokens"]), x.get("value_token", "")))
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--upstream-dir", required=True)
    ap.add_argument("--schema-file", required=True)
    args = ap.parse_args()

    upstream = Path(args.upstream_dir)
    schema_file = Path(args.schema_file)
    transclude = upstream / "scripts" / "transclude-template"
    defs_dir = upstream / "interface-definitions"

    current = json.loads(schema_file.read_text())
    generated: list[dict] = []
    for f in sorted(defs_dir.glob("*.xml.in")):
        try:
            generated.extend(parse_definitions(transclude, f))
        except Exception:
            continue

    merged = [normalize_entry(e) for e in current]
    merged.extend(normalize_entry(e) for e in generated)
    merged = dedupe_sort(merged)
    schema_file.write_text(json.dumps(merged, indent=2) + "\n")

    set_count = sum(1 for x in merged if x.get("kind") == "set")
    show_count = sum(1 for x in merged if x.get("kind") == "show")
    print(json.dumps({"set": set_count, "show": show_count, "total": len(merged)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
