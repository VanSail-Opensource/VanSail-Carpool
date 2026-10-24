"""Pipe pgbackrest --output=json info here; publish atomically via node_exporter."""
import json
import sys

for stanza in json.load(sys.stdin):
    label = json.dumps(stanza["name"])
    latest = max((b["timestamp"]["stop"] for b in stanza.get("backup", [])), default=0)
    print(f'vansail_pgbackrest_last_backup_timestamp_seconds{{stanza={label}}} {latest}')
    print(f'vansail_pgbackrest_status{{stanza={label}}} {stanza["status"]["code"]}')
