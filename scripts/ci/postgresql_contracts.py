"""Select repository contracts; historical SQLite upgrade fixtures stay SQLite-only."""

import json
import re
import sys
from pathlib import Path

selected, excluded = [], []
for path in sorted(Path("internal/platform/database").glob("*_test.go")):
    source = path.read_text(encoding="utf-8")
    functions = list(re.finditer(r"^func (\w+)\(", source, re.M))
    for index, match in enumerate(functions):
        name = match.group(1)
        if not name.startswith("Test"):
            continue
        end = functions[index + 1].start() if index + 1 < len(functions) else len(source)
        body = source[match.start():end]
        if re.search(r"Migration|SQLite|SchemaSignature|RestoreValidation|TelegramIdentity", name) or re.search(
            r'\bPRAGMA\b|pragma_\w+|sqlite_(?:schema|master)|migrations\.ReadFile|sql\.Open\(|\bOpen\(ctx|CompleteOutboxJob\(|SET username|INSERT INTO users\([^)]*\busername\b', body
        ):
            excluded.append({"test": name, "reason": "SQLite-specific source/upgrade fixture"})
        else:
            selected.append(name)
Path(sys.argv[1]).write_text("^(" + "|".join(selected) + ")$\n", encoding="utf-8")
Path(sys.argv[2]).write_text(json.dumps({"selected": selected, "sqliteOnly": excluded}, indent=2) + "\n", encoding="utf-8")
print(f"PostgreSQL contracts: {len(selected)}; SQLite upgrade/maintenance fixtures: {len(excluded)}")
