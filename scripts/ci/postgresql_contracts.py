"""Select portable contracts, tracing helper calls into SQLite upgrade fixtures."""

import json
import re
import sys
from pathlib import Path

functions = {}
for path in sorted(Path("internal/platform/database").glob("*_test.go")):
    source = path.read_text(encoding="utf-8")
    matches = list(re.finditer(r"^func (\w+)\(", source, re.M))
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(source)
        functions[match[1]] = (str(path), source[match.start():end])

# The common factory deliberately chooses its backend at runtime.
adapters = {"newTestStore", "openTestDatabase", "openPostgresContractDatabase"}
pattern = re.compile(r'\bPRAGMA\b|pragma_\w+|sqlite_(?:schema|master)|migrations\.ReadFile|sql\.Open\(|\bOpen\(ctx|CompleteOutboxJob\(|SET username|INSERT INTO users\([^)]*\busername\b|(?:ReadFile|Join)\("migrations[/"]')
blocked = {name for name, (path, body) in functions.items()
           if name not in adapters and (pattern.search(body) or
               name.startswith("Test") and (re.search(r"Migration|SQLite|SchemaSignature|RestoreValidation|TelegramIdentity", name) or "_migration_test.go" in path))}
while True:
    added = {name for name, (_, body) in functions.items()
             if name not in blocked | adapters and any(re.search(r"\b" + callee + r"\(", body) for callee in blocked)}
    if not added:
        break
    blocked.update(added)
selected = sorted(name for name in functions if name.startswith("Test") and name not in blocked)
excluded = [{"test": name, "reason": "SQLite-specific fixture or helper dependency"}
            for name in sorted(blocked) if name.startswith("Test")]
Path(sys.argv[1]).write_text("^(" + "|".join(selected) + ")$\n", encoding="utf-8")
Path(sys.argv[2]).write_text(json.dumps({"selected": selected, "sqliteOnly": excluded}, indent=2) + "\n", encoding="utf-8")
print(f"PostgreSQL contracts: {len(selected)}; SQLite upgrade/maintenance fixtures: {len(excluded)}")
