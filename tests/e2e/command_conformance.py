#!/usr/bin/env python3
import argparse
import json
import subprocess
import tempfile
from datetime import datetime, timezone
from pathlib import Path


def sample_for_token(tok: str) -> str:
    if tok == "<ifname>":
        return "eth0"
    if tok == "<id>":
        return "100"
    if tok == "<name>":
        return "edge"
    if tok.startswith("<") and tok.endswith(">"):
        return "value"
    return tok


def sample_for_value(tok: str) -> str:
    m = {
        "": "value",
        "<name>": "edge",
        "<server>": "0.pool.ntp.org",
        "<ifname>": "eth0",
        "<id>": "100",
        "<port>": "443",
        "<cidr>": "10.0.0.0/24",
        "<ip>": "10.0.0.1",
        "<ipv4>": "10.0.0.1",
        "<ipv6>": "2001:db8::1",
        "<fqdn>": "api.example.com",
        "<value>": "example",
        "<allow|deny>": "deny",
        "<accept|drop>": "drop",
        "<tcp|udp|all>": "tcp",
        "<tcp|udp|icmp|all>": "tcp",
        "<tcp|http>": "http",
        "<lan|wan>": "wan",
    }
    if tok in m:
        return m[tok]
    if tok.startswith("<") and tok.endswith(">"):
        return "value"
    return tok


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--schema", required=True)
    ap.add_argument("--binary", required=True)
    ap.add_argument("--report", required=True)
    args = ap.parse_args()

    schema = json.loads(Path(args.schema).read_text())
    set_paths = [x for x in schema if x.get("kind") == "set"]

    report = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "schema_file": args.schema,
        "binary": args.binary,
        "set_total": len(set_paths),
        "set_passed": 0,
        "set_failed": 0,
        "failures": [],
    }

    with tempfile.TemporaryDirectory() as td:
        candidate = str(Path(td) / "candidate.json")
        for entry in set_paths:
            tokens = [sample_for_token(t) for t in entry["tokens"]]
            value = sample_for_value(entry.get("value_token", ""))
            cmd = [args.binary, "set", "--file", candidate, *tokens, value]
            proc = subprocess.run(cmd, capture_output=True, text=True)
            if proc.returncode == 0:
                report["set_passed"] += 1
            else:
                report["set_failed"] += 1
                report["failures"].append(
                    {
                        "tokens": entry["tokens"],
                        "value_token": entry.get("value_token", ""),
                        "stdout": proc.stdout,
                        "stderr": proc.stderr,
                        "returncode": proc.returncode,
                    }
                )

    Path(args.report).parent.mkdir(parents=True, exist_ok=True)
    Path(args.report).write_text(json.dumps(report, indent=2) + "\n")

    print(json.dumps({"set_total": report["set_total"], "set_failed": report["set_failed"]}))
    return 0 if report["set_failed"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
