"""Run one hosted race shard, or verify artifacts from all eight shards."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from race_inventory import SHARD_COUNT, discover, verify


def run_check(arguments, output, environment):
    print("Running " + " ".join(arguments), flush=True)
    with subprocess.Popen(arguments, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          text=True, env=environment) as process:
        for line in process.stdout:
            output.write(line)
            output.flush()
            sys.stdout.write(line)
        if process.wait() != 0:
            raise RuntimeError("race check failed")


def run_shard(index, directory):
    inventory = discover()
    manifest = {**inventory["partitions"][index], "allTests": inventory["allTests"],
                "allPackages": inventory["allPackages"], "completed": False}
    directory.mkdir(parents=True, exist_ok=True)
    manifest_path = directory / f"race-inventory-{index}.json"
    manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
    cpus = len(os.sched_getaffinity(0)) if hasattr(os, "sched_getaffinity") else os.cpu_count() or 1
    environment = {**os.environ, "GOMAXPROCS": str(cpus)}
    flags = ["go", "test", "-race", "-gcflags=github.com/xtls/xray-core/proxy/vless/...=-d=checkptr=0", "-count=1", "-timeout", "20m", "-json",
             "-p", str(cpus), "-parallel", str(cpus)]
    with (directory / f"race-results-{index}.json").open("w", encoding="utf-8") as output:
        if manifest["packages"]:
            run_check(flags + manifest["packages"], output, environment)
        if manifest["tests"]:
            pattern = "^(" + "|".join(re.escape(test) for test in manifest["tests"]) + ")$"
            run_check(flags + ["-run", pattern, inventory["database"]], output, environment)
    manifest["completed"] = True
    manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
    print(f"Shard {index + 1}/{SHARD_COUNT}: {len(manifest['tests'])} database checks, "
          f"{len(manifest['packages'])} other packages", flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--shard", type=int, choices=range(SHARD_COUNT))
    parser.add_argument("--verify", action="store_true")
    parser.add_argument("--directory", type=Path, required=True)
    arguments = parser.parse_args()
    if arguments.verify:
        manifests = [json.loads(path.read_text(encoding="utf-8"))
                     for path in arguments.directory.glob("race-inventory-*.json")]
        verify(manifests)
        print(f"All Go packages and database checks completed exactly once across {SHARD_COUNT} shards.")
    elif arguments.shard is not None:
        run_shard(arguments.shard, arguments.directory)
    else:
        parser.error("choose --shard or --verify")


if __name__ == "__main__":
    main()
