"""Discover and partition Go race checks without executing test bodies."""

import re
import subprocess

DATABASE_PACKAGE = "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
SHARD_COUNT = 8


def discover():
    packages = sorted(subprocess.check_output(["go", "list", "./..."], text=True).splitlines())
    listing = subprocess.check_output(
        ["go", "test", "-race", "-list", ".", DATABASE_PACKAGE], text=True
    )
    tests = sorted(set(line for line in listing.splitlines()
                       if re.fullmatch(r"(?:Test|Example|Fuzz)\w*", line)))
    if DATABASE_PACKAGE not in packages or not tests:
        raise RuntimeError("database race inventory is empty")
    others = [package for package in packages if package != DATABASE_PACKAGE]
    partitions = [
        {"shard": index, "tests": tests[index::SHARD_COUNT],
         "packages": others[index::SHARD_COUNT]}
        for index in range(SHARD_COUNT)
    ]
    if sorted(test for partition in partitions for test in partition["tests"]) != tests:
        raise RuntimeError("database tests were omitted or assigned more than once")
    if sorted(package for partition in partitions for package in partition["packages"]) != others:
        raise RuntimeError("Go packages were omitted or assigned more than once")
    return {"database": DATABASE_PACKAGE, "allTests": tests, "allPackages": others,
            "partitions": partitions}


def verify(manifests):
    if len(manifests) != SHARD_COUNT or sorted(item["shard"] for item in manifests) != list(range(SHARD_COUNT)):
        raise RuntimeError(f"expected {SHARD_COUNT} distinct completed race shards")
    reference = manifests[0]
    for manifest in manifests:
        if manifest["allTests"] != reference["allTests"] or manifest["allPackages"] != reference["allPackages"]:
            raise RuntimeError("race shards discovered different inventories")
        if not manifest.get("completed"):
            raise RuntimeError("a race shard did not complete")
    if sorted(test for item in manifests for test in item["tests"]) != reference["allTests"]:
        raise RuntimeError("database race coverage contains omissions or duplicates")
    if sorted(package for item in manifests for package in item["packages"]) != reference["allPackages"]:
        raise RuntimeError("package race coverage contains omissions or duplicates")
