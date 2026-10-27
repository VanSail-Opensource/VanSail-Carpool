"""Preview or apply proven period aliases from an immutable SQLite snapshot.

Uses psql and the caller's PostgreSQL environment, or an embedded container.
Run with --help. The default is read-only; --apply writes one audited transaction.
"""

import argparse
import hashlib
import json
import sqlite3
import subprocess
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path


def literal(value):
    if value is None:
        return "NULL"
    return "'" + str(value).replace("'", "''") + "'"


def psql(command, statement):
    result = subprocess.run(command, input=statement, text=True, capture_output=True, check=False)
    if result.returncode:
        raise RuntimeError("PostgreSQL recovery query failed: " + result.stderr.strip())
    return result.stdout


def select_json(command, query):
    statement = "BEGIN READ ONLY; SET LOCAL statement_timeout='30s'; SELECT row_to_json(q) FROM (" + query + ") q; COMMIT;"
    return [json.loads(line) for line in psql(command, statement).splitlines() if line.startswith("{")]


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def preview(snapshot, node, command):
    source = sqlite3.connect(snapshot.as_uri() + "?mode=ro&immutable=1", uri=True)
    source.row_factory = sqlite3.Row
    try:
        agents = list(source.execute("SELECT agent_id FROM xnode_nodes WHERE id=?", (node,)))
        if len(agents) != 1 or not agents[0][0]:
            raise ValueError("snapshot does not identify one bound node agent")
        old = [dict(row) for row in source.execute("""SELECT p.*, (SELECT COUNT(*) FROM xnode_daily_usage d
            WHERE d.period_id=p.id AND (d.upload_bytes<>0 OR d.download_bytes<>0 OR d.weighted_bytes<>0)) AS recorded
            FROM xnode_periods p JOIN xnode_period_nodes n ON n.period_id=p.id WHERE n.node_id=?""", (node,))]
    finally:
        source.close()
    live_agent = select_json(command, "SELECT agent_id FROM xnode_nodes WHERE id=" + literal(node))
    if len(live_agent) != 1 or live_agent[0]["agent_id"] != agents[0][0]:
        raise ValueError("snapshot and target node agent differ")
    live = select_json(command, "SELECT id,proxy_user_id,purchase_id,started_at,last_reset_at,limit_bytes,strategy,status FROM xnode_periods")
    present = {row["id"] for row in live}
    fields = ("proxy_user_id", "purchase_id", "started_at", "last_reset_at", "limit_bytes", "strategy")
    mappings = []
    for historical in old:
        if historical["id"] in present:
            continue
        if historical["used_bytes"] != 0 or historical["recorded"] != 0 or not historical["purchase_id"] or historical["strategy"] == "PAYG":
            raise ValueError("historical period has usage or unsupported ownership: " + historical["id"])
        matches = [row for row in live if all(row[field] == historical[field] for field in fields)]
        if len(matches) != 1 or matches[0]["status"] not in ("open", "closing"):
            raise ValueError("period has no unique unsettled match: " + historical["id"])
        mappings.append({"historical": historical["id"], "canonical": matches[0]["id"],
                         **{field: historical[field] for field in fields}})
    return agents[0][0], sorted(mappings, key=lambda row: row["historical"])


def apply(command, node, agent, digest, mappings):
    if not mappings:
        return
    values = []
    for item in mappings:
        values.append("(" + ",".join(literal(item[k]) for k in ("historical", "canonical", "proxy_user_id", "purchase_id", "started_at", "last_reset_at", "limit_bytes", "strategy")) + ")")
    detail = json.dumps({"nodeId": node, "agentId": agent, "snapshotSha256": digest, "mappings": mappings}, separators=(",", ":"))
    stamp = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    sql = """BEGIN ISOLATION LEVEL SERIALIZABLE;
SET LOCAL lock_timeout='10s'; SET LOCAL statement_timeout='60s';
CREATE TEMP TABLE period_recovery_input(historical text,canonical text,owner bigint,purchase text,started text,reset_at text,quota bigint,strategy text) ON COMMIT DROP;
INSERT INTO period_recovery_input VALUES """ + ",".join(values) + ";\n"
    sql += "SELECT id FROM xnode_periods WHERE id IN (SELECT canonical FROM period_recovery_input) ORDER BY id FOR UPDATE;\n"
    sql += """DO $$ BEGIN
IF NOT EXISTS(SELECT 1 FROM xnode_nodes WHERE id=""" + literal(node) + " AND agent_id=" + literal(agent) + """ ) THEN RAISE EXCEPTION 'node identity changed'; END IF;
IF EXISTS(SELECT 1 FROM period_recovery_input i WHERE EXISTS(SELECT 1 FROM xnode_periods WHERE id=i.historical)
 OR (SELECT COUNT(*) FROM xnode_periods p WHERE p.proxy_user_id=i.owner AND p.purchase_id=i.purchase AND p.started_at=i.started AND p.last_reset_at IS NOT DISTINCT FROM i.reset_at AND p.limit_bytes=i.quota AND p.strategy=i.strategy)<>1
 OR NOT EXISTS(SELECT 1 FROM xnode_periods p WHERE p.id=i.canonical AND p.proxy_user_id=i.owner AND p.purchase_id=i.purchase AND p.started_at=i.started AND p.last_reset_at IS NOT DISTINCT FROM i.reset_at AND p.limit_bytes=i.quota AND p.strategy=i.strategy AND p.status IN ('open','closing'))
 OR EXISTS(SELECT 1 FROM xnode_period_nodes n WHERE n.period_id=i.canonical AND n.node_id=""" + literal(node) + """ AND n.flushed_sequence IS NOT NULL)
 OR EXISTS(SELECT 1 FROM xnode_period_aliases a WHERE a.node_id=""" + literal(node) + """ AND a.historical_period_id=i.historical AND a.canonical_period_id<>i.canonical)) THEN RAISE EXCEPTION 'recovery precondition changed'; END IF;
END $$;
"""
    sql += "INSERT INTO xnode_period_nodes(period_id,node_id) SELECT canonical," + literal(node) + " FROM period_recovery_input ON CONFLICT(period_id,node_id) DO NOTHING;\n"
    sql += "INSERT INTO xnode_period_aliases(node_id,historical_period_id,canonical_period_id) SELECT " + literal(node) + ",historical,canonical FROM period_recovery_input ON CONFLICT(node_id,historical_period_id) DO NOTHING;\n"
    sql += "INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(" + ",".join(map(literal, (str(uuid.uuid4()), None, "xnode_period_recovery", "xnode_node", node, detail, stamp))) + ");\nCOMMIT;"
    psql(command, sql)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--snapshot", type=Path, required=True)
    parser.add_argument("--snapshot-sha256", required=True)
    parser.add_argument("--node", required=True)
    parser.add_argument("--docker-container", help="Run psql inside an embedded PostgreSQL panel container")
    parser.add_argument("--database", default="vansail", help="Target database; use a restored copy for rehearsal")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    node = str(uuid.UUID(args.node))
    snapshot = args.snapshot.resolve(strict=True)
    digest = sha256(snapshot)
    if digest != args.snapshot_sha256.lower():
        raise ValueError("snapshot SHA-256 does not match")
    command = ["psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-d", args.database]
    if args.docker_container:
        command = ["docker", "exec", "-i", args.docker_container] + command + ["-U", "postgres", "-h", "/var/lib/postgresql/socket"]
    agent, mappings = preview(snapshot, node, command)
    print(json.dumps({"mode": "apply" if args.apply else "preview", "snapshotSha256": digest, "nodeId": node, "agentId": agent, "mappings": mappings}, indent=2))
    if args.apply:
        apply(command, node, agent, digest, mappings)


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, ValueError, sqlite3.Error) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
