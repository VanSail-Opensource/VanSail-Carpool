"""Generate a source-location inventory; does not run application tests or SQL."""

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DESTINATION = ROOT / "docs/plans/postgresql"
PATTERNS = {
    "sqlite-maintenance": r"PRAGMA|sqlite_schema|sqlite_master|pragma_\w+|VACUUM",
    "implicit-row-identity": r"\browid\b|LastInsertId",
    "json": r"\bjson_(?:extract|valid|set|each)\b|\bjson\(",
    "time": r"\bjulianday\b|\bstrftime\b|\bdatetime\(",
    "conflict": r"INSERT OR|unique constraint|constraint failed|SQLITE_",
    "comparison": r"\bGLOB\b|COLLATE NOCASE|\bIS \?|\bMAX\(|\bMIN\(",
    "sql-contract": r"\*sql\.(?:DB|Tx|Conn)\b",
    "writer-serialization": r"writeMu\.(?:Lock|Unlock)",
    "transaction": r"\.BeginTx\(",
    "outbox-atomicity": r"enqueue\w*Tx\(|INSERT (?:OR IGNORE )?INTO outbox_jobs",
}


def main():
    references = []
    for path in sorted((ROOT / "internal").rglob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        function = "package scope"
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if line.startswith("func "):
                function = line.split("{")[0].strip()
            categories = [key for key, pattern in PATTERNS.items() if re.search(pattern, line, re.I)]
            if categories:
                references.append({"file": path.relative_to(ROOT).as_posix(), "line": number,
                                   "function": function, "categories": categories})
    DESTINATION.mkdir(parents=True, exist_ok=True)
    (DESTINATION / "source-inventory.json").write_text(
        json.dumps(references, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
