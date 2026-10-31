"""A small delivery receipt journal and bounded read-only index inspection.

The journal is not an indexing queue. Fabric retries durable publications; this
module records an upload before transmission and reconciles uncertain outcomes.
All index mutations go through the existing WeKnora API.
"""
import json
import sqlite3
from contextlib import contextmanager
from pathlib import Path
import psycopg
from psycopg.rows import dict_row

class Journal:
    def __init__(self, path):
        self.path = path
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        with self.connect() as db:
            db.execute("""CREATE TABLE IF NOT EXISTS publications (
                id TEXT PRIMARY KEY, payload TEXT NOT NULL, knowledge_id TEXT,
                upload_status TEXT NOT NULL, source_status TEXT NOT NULL DEFAULT 'active',
                disable_reason TEXT, created_at TEXT DEFAULT CURRENT_TIMESTAMP)""")
        Path(path).chmod(0o600)

    def connect(self):
        db = sqlite3.connect(self.path, timeout=15)
        db.row_factory = sqlite3.Row
        return db

    def get(self, pid):
        with self.connect() as db:
            row = db.execute("SELECT * FROM publications WHERE id=?", (pid,)).fetchone()
        if row:
            result = dict(row)
            result["payload"] = json.loads(result["payload"])
            return result

    def reserve(self, pid, payload):
        with self.connect() as db:
            cur = db.execute("INSERT OR IGNORE INTO publications(id,payload,upload_status) VALUES(?,?,?)",
                             (pid, json.dumps(payload, ensure_ascii=False), "sending"))
            return cur.rowcount == 1

    def update(self, pid, **values):
        allowed = {"knowledge_id", "upload_status", "source_status", "disable_reason"}
        if not values or set(values) - allowed:
            raise ValueError("invalid receipt update")
        with self.connect() as db:
            db.execute("UPDATE publications SET " + ",".join(k + "=?" for k in values) + " WHERE id=?",
                       (*values.values(), pid))

    def all(self):
        with self.connect() as db:
            return [r[0] for r in db.execute("SELECT id FROM publications ORDER BY created_at,id")]

class IndexStore:
    def __init__(self, config):
        self.config = config

    @contextmanager
    def connect(self):
        # A read-only transaction is mandatory even if an operator supplied an
        # existing database account. No SQL write path exists in this module.
        with psycopg.connect(**self.config, connect_timeout=5, row_factory=dict_row,
                             options="-c default_transaction_read_only=on -c statement_timeout=15000") as db:
            with db.transaction():
                db.execute("SET TRANSACTION READ ONLY")
                yield db

    def document(self, kb, kid):
        with self.connect() as db:
            doc = db.execute("""SELECT id,knowledge_base_id,tenant_id,embedding_model_id,
                parse_status,enable_status,metadata,custom_metadata,processed_at,updated_at
                FROM knowledges WHERE id=%s AND knowledge_base_id=%s AND deleted_at IS NULL""", (kid,kb)).fetchone()
            if not doc:
                return None, []
            rows = db.execute("""SELECT c.id,c.content,c.chunk_index,c.chunk_type,c.is_enabled,
                c.start_at,c.end_at,c.content_revision,c.index_status,c.updated_at,
                e.dimension,e.content AS embedded_text,e.embedding::text AS embedding,e.is_enabled AS vector_enabled,
                e.updated_at AS vector_updated_at
                FROM chunks c LEFT JOIN embeddings e ON e.chunk_id=c.id
                    AND e.knowledge_id=c.knowledge_id AND e.knowledge_base_id=c.knowledge_base_id
                    AND e.source_type=0 AND e.source_id=c.id
                WHERE c.knowledge_id=%s AND c.knowledge_base_id=%s AND c.tenant_id=%s
                    AND c.deleted_at IS NULL
                ORDER BY c.chunk_index,c.id LIMIT 2001""", (kid,kb,doc["tenant_id"])).fetchall()
            if len(rows) > 2000:
                raise ValueError("document exceeds bounded chunk limit")
            return doc, rows

    def model(self, model_id):
        with self.connect() as db:
            return db.execute("""SELECT id,name,type,source,parameters,parameters->>'provider' AS provider,
                parameters->'embedding_parameters'->>'dimension' AS dimension,updated_at
                FROM models WHERE id=%s AND deleted_at IS NULL""", (model_id,)).fetchone()
