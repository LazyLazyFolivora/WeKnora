"""Fixed-version publication and feature facade over the deployed WeKnora API."""
import hashlib
import json
import math
import threading
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import quote

import httpx
from fastapi import HTTPException
from .storage import Journal, IndexStore

CONTRACT = "weknora-kdd-v1"
IDENTITY_FIELDS = ("source_instance", "source_id", "source_record_id", "version_id", "raw_sha256")
AGGREGATION = "mean_l2_normalized_enabled_text_chunks_v1"

def digest(value):
    if not isinstance(value, bytes):
        value = json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(value).hexdigest()

def identity(value):
    return {k: value[k] for k in IDENTITY_FIELDS}

def publication_id(value):
    return digest([value["knowledge_base_id"], *[value[k] for k in IDENTITY_FIELDS[:-1]]])

def model_descriptor(model):
    params = model.get("parameters", {})
    stamp = model["updated_at"]
    if isinstance(stamp, str):
        stamp = datetime.fromisoformat(stamp.replace("Z", "+00:00"))
    stamp = stamp.astimezone(timezone.utc).isoformat()
    out = {"model_id": model["id"], "model_name": model["name"],
           "provider": model.get("provider") or params.get("provider") or model["source"],
           "dimension": int(model.get("dimension") or params.get("embedding_parameters", {}).get("dimension", 0)),
           "configuration_updated_at": stamp, "artifact_version": None}
    out["model_version"] = "configuration:" + digest(out)
    return out

def aggregate(vectors):
    if not vectors or not vectors[0]:
        raise ValueError("no vectors")
    dim = len(vectors[0])
    total = [0.0] * dim
    for vector in vectors:
        if len(vector) != dim or any(not math.isfinite(x) for x in vector):
            raise ValueError("invalid vector dimensions or values")
        norm = math.sqrt(sum(x*x for x in vector))
        if not norm:
            raise ValueError("zero vector")
        for i, value in enumerate(vector):
            total[i] += value / norm / len(vectors)
    norm = math.sqrt(sum(x*x for x in total))
    if not norm:
        raise ValueError("zero aggregate")
    return [x / norm for x in total]

class Service:
    def __init__(self, config_path, client=None, store=None):
        self.config_path = Path(config_path)
        self.cfg = self.config()
        self.journal = Journal(self.cfg["journal_path"])
        self.store = store or IndexStore(self.cfg["db"])
        self.client = client or httpx.Client(timeout=45, follow_redirects=False)
        self.write_lock = threading.RLock()

    def config(self):
        return json.loads(self.config_path.read_text())

    def authorize(self, kb, scope, source=None):
        # Reload the source mapping so a permission removal applies immediately.
        cfg = self.config()
        if kb not in cfg["allowed_kb_ids"]:
            raise HTTPException(403, "knowledge base is outside configured access")
        mappings = [m for m in cfg["source_mappings"] if m["knowledge_base_id"] == kb and scope in m["scopes"]]
        if source is not None:
            mappings = [m for m in mappings if m["source_instance"] == source["source_instance"] and m["source_id"] == source["source_id"]]
        if not mappings:
            raise HTTPException(403, "source and scope are outside configured access")

    def request(self, method, path, **kwargs):
        try:
            response = self.client.request(method, self.cfg["upstream_url"] + path,
                headers={"X-API-Key": self.cfg["upstream_api_key"]}, **kwargs)
        except httpx.HTTPError:
            raise HTTPException(502, "WeKnora request outcome unavailable") from None
        if response.status_code not in (200,201,202,204,409):
            raise HTTPException(502, f"WeKnora returned HTTP {response.status_code}")
        try:
            result = response.json()
        except ValueError:
            raise HTTPException(502, "WeKnora returned an invalid response") from None
        if response.status_code == 409:
            return result
        if result.get("success") is False:
            raise HTTPException(502, "WeKnora rejected operation")
        return result

    def get_document(self, kid):
        return self.request("GET", "/api/v1/knowledge/" + quote(kid, safe=""))["data"]

    def configured_model(self, kind):
        model_id = "builtin-embedding-default" if kind == "embedding" else "builtin-rerank-default"
        return next(m for m in self.cfg["models"] if m["id"] == model_id)

    def check_model(self, kind):
        configured = self.configured_model(kind)
        current = self.store.model(configured["id"])
        if not current or model_descriptor(current) != model_descriptor(configured):
            raise HTTPException(409, "model configuration changed; refresh the pinned service configuration")
        return configured

    def prepare(self, req):
        value = req.model_dump(exclude_none=True)
        self.authorize(value["knowledge_base_id"], value["source_scope"], value)
        if len(value["content"].encode()) > 2000000:
            raise HTTPException(400, "content exceeds byte limit")
        for k in IDENTITY_FIELDS[:-1]:
            if value["raw_locator"][k] != value[k]:
                raise HTTPException(400, "raw locator does not match the fixed identity")
        checksum = digest(value["content"].encode())
        if value.get("content_sha256") and checksum != value["content_sha256"]:
            raise HTTPException(400, "content_sha256 mismatch")
        value["content_sha256"] = checksum
        return value

    def metadata(self, value, pid):
        return {**identity(value), "source_scope": value["source_scope"],
                "raw_locator": json.dumps(value["raw_locator"], ensure_ascii=False, separators=(",", ":")),
                "publication_id": pid, "content_sha256": value["content_sha256"],
                "indexed_text_sha256": value["indexed_text_sha256"], "source_status": "active",
                "contract_version": CONTRACT}

    def recover(self, row):
        value, pid = row["payload"], row["id"]
        response = self.request("GET", "/api/v1/knowledge-bases/" + value["knowledge_base_id"] + "/knowledge",
                                params={"keyword": "fabric-" + pid, "page": 1, "page_size": 10})
        matches = []
        for doc in response.get("data", []):
            meta = doc.get("metadata") or {}
            if meta.get("publication_id") == pid and meta.get("content_sha256") == value["content_sha256"]:
                matches.append(doc)
        if len(matches) == 1:
            self.journal.update(pid, knowledge_id=matches[0]["id"], upload_status="uploaded")
            return self.journal.get(pid)
        raise HTTPException(409, "upload outcome unknown; reconciliation found no unique fixed version; no resend performed")

    def publish(self, req):
        value = self.prepare(req)
        pid = publication_id(value)
        with self.write_lock:
            row = self.journal.get(pid)
            if row:
                prior = row["payload"]
                for k in ("raw_sha256", "content_sha256", "source_scope", "raw_locator"):
                    if value[k] != prior[k]:
                        raise HTTPException(409, "fixed version already exists with different identity/content/scope")
                if not row["knowledge_id"]:
                    row = self.recover(row)
                self.ensure_binding(row)
                return self.status(pid)
            kb = self.request("GET", "/api/v1/knowledge-bases/" + value["knowledge_base_id"])["data"]
            model = self.check_model("embedding")
            if kb["embedding_model_id"] != model["id"]:
                raise HTTPException(409, "knowledge base embedding model differs from the pinned model")
            value["embedding_provenance"] = model_descriptor(model)
            value["chunking"] = kb["chunking_config"]
            indexed = "<!-- Fabric fixed version " + pid + " -->\n" + value["content"]
            value["indexed_text_sha256"] = digest(indexed.encode())
            if not self.journal.reserve(pid, value):
                raise HTTPException(409, "publication is already being delivered")
            metadata = self.metadata(value, pid)
            try:
                result = self.request("POST", "/api/v1/knowledge-bases/" + value["knowledge_base_id"] + "/knowledge/file",
                    files={"file": ("fabric-" + pid + ".md", indexed.encode(), "text/markdown")},
                    data={"metadata": json.dumps(metadata, ensure_ascii=False), "channel": "api",
                          "process_config": json.dumps({"graph_enabled": False, "extract_config": {"enabled": False},
                              "question_generation_config": {"enabled": False, "question_count": 0},
                              "chunking_config": value["chunking"]})})
                doc = result.get("data") or result.get("knowledge")
                if not isinstance(doc, dict) or not doc.get("id"):
                    raise HTTPException(502, "upload returned no knowledge ID")
                kid = doc["id"]
                check = self.get_document(kid)
                if (check.get("metadata") or {}).get("publication_id") != pid:
                    raise HTTPException(409, "upstream duplicate does not match the fixed source version")
                self.journal.update(pid, knowledge_id=kid, upload_status="uploaded")
            except HTTPException:
                if not self.journal.get(pid)["knowledge_id"]:
                    self.journal.update(pid, upload_status="unknown")
                raise
            self.ensure_binding(self.journal.get(pid))
            return self.status(pid)

    def ensure_binding(self, row):
        if not row["knowledge_id"]:
            return
        value = row["payload"]
        doc = self.get_document(row["knowledge_id"])
        expected = self.metadata(value, row["id"])
        if row["source_status"] != "active":
            expected["source_status"] = row["source_status"]
        meta = doc.get("metadata") or {}
        if any(meta.get(k) != expected[k] for k in IDENTITY_FIELDS):
            raise HTTPException(409, "upstream fixed source metadata mismatch")
        custom = doc.get("custom_metadata") or {}
        # Reserve the contract fields in custom_metadata, which native clients
        # already receive. Original metadata was stored atomically at upload.
        update = {**custom, **expected}
        if len(update) > 20 or any(len(str(v)) > 1000 for v in update.values()):
            raise HTTPException(409, "upstream custom metadata size limit exceeded")
        if custom != update:
            self.request("PUT", "/api/v1/knowledge/" + row["knowledge_id"],
                         json={"title": value["title"], "custom_metadata": update})

    def row(self, pid, receipt_only=False):
        row = self.journal.get(pid)
        if row is None:
            raise HTTPException(404, "publication not found")
        value = row["payload"]
        if not receipt_only:
            self.authorize(value["knowledge_base_id"], value["source_scope"], value)
        # Receipt/cleanup access is confined to bindings this authenticated
        # service previously registered. It reveals no document content and
        # remains available after a source mapping has been revoked.
        return row

    def inspect(self, row):
        value = row["payload"]
        if not row["knowledge_id"]:
            return None, [], ["upload_not_confirmed"]
        try:
            authorized_doc = self.get_document(row["knowledge_id"])
            if authorized_doc["knowledge_base_id"] != value["knowledge_base_id"]:
                return None, [], ["source_binding_mismatch"]
            doc, chunks = self.store.document(value["knowledge_base_id"], row["knowledge_id"])
            if doc and doc["tenant_id"] != authorized_doc["tenant_id"]:
                return None, [], ["source_binding_mismatch"]
        except HTTPException:
            return None, [], ["upstream_access_unavailable"]
        except Exception:
            return None, [], ["vector_store_unavailable"]
        if doc is None:
            return None, [], ["upstream_document_missing"]
        expected = self.metadata(value, row["id"])
        meta = doc.get("metadata") or {}
        custom = doc.get("custom_metadata") or {}
        reasons = []
        binding_fields = (*IDENTITY_FIELDS, "publication_id", "raw_locator", "source_scope", "content_sha256", "indexed_text_sha256")
        if any(meta.get(k) != expected[k] or custom.get(k) != expected[k] for k in binding_fields):
            reasons.append("source_binding_mismatch")
        if any(c["content_revision"] != 0 for c in chunks):
            reasons.append("indexed_chunk_content_changed")
        if len({c["id"] for c in chunks}) != len(chunks):
            reasons.append("duplicate_chunk_vectors")
        if meta.get("content_sha256") != value["content_sha256"] or meta.get("indexed_text_sha256") != value["indexed_text_sha256"]:
            reasons.append("index_text_mismatch")
        if doc["embedding_model_id"] != value["embedding_provenance"]["model_id"]:
            reasons.append("embedding_model_binding_changed")
        if row["source_status"] != "active" or custom.get("source_status") != "active" or doc["enable_status"] != "enabled":
            reasons.append("source_disabled")
        return doc, chunks, reasons

    def status(self, pid):
        row = self.row(pid, receipt_only=True)
        value = row["payload"]
        doc, chunks, reasons = self.inspect(row)
        text_chunks = [c for c in chunks if c["chunk_type"] == "text"]
        vectors = [c for c in text_chunks if c["is_enabled"] and c["vector_enabled"] and c["embedding"]]
        parse = doc["parse_status"] if doc else "unknown"
        ready = bool(text_chunks) and len(vectors) == len(text_chunks) and parse in ("completed", "finalizing") and not reasons
        disabled = row["source_status"] != "active"
        confirmed_disabled = disabled and doc is not None and all(not c["is_enabled"] and not c["vector_enabled"] for c in chunks)
        vector_status = "ready" if ready else "pending"
        index_status = "ready" if ready else "pending"
        if {"vector_store_unavailable", "upstream_access_unavailable"}.intersection(reasons):
            vector_status = index_status = "unavailable"
        elif confirmed_disabled:
            vector_status = index_status = "disabled"
        elif parse in ("failed", "cancelled") and not disabled:
            index_status = "failed"
        elif parse == "completed" and not vectors and not disabled:
            vector_status = "missing"
        version = digest([row["knowledge_id"], value["indexed_text_sha256"], value["embedding_provenance"], value["chunking"],
                          [(c["id"],str(c["updated_at"]),str(c["vector_updated_at"]),c["is_enabled"],c["vector_enabled"]) for c in chunks]])
        return {"contract_version": CONTRACT, "publication_id": pid, "knowledge_id": row["knowledge_id"],
                "knowledge_base_id": value["knowledge_base_id"], "identity": identity(value), "raw_locator": value["raw_locator"],
                "source_scope": value["source_scope"], "content_sha256": value["content_sha256"],
                "indexed_text_sha256": value["indexed_text_sha256"], "upload_status": row["upload_status"],
                "parse_status": parse, "vector_status": vector_status, "index_status": index_status,
                "binding_status": "bound" if doc and not {"source_binding_mismatch", "index_text_mismatch"}.intersection(reasons) else "pending",
                "source_status": "disabled" if disabled else "active", "source_change_type": row["source_status"],
                "disable_reason": row["disable_reason"], "chunk_count": len(chunks), "vector_count": len(vectors),
                "index_version": version, "graph_status": "not_requested", "degraded_reasons": reasons}

    def feature(self, row, include_chunks=False):
        value = row["payload"]
        status = self.status(row["id"])
        doc, chunks, reasons = self.inspect(row)
        provenance = {**value["embedding_provenance"], "verified": False}
        text_chunks = [c for c in chunks if c["chunk_type"] == "text"]
        vectors = []
        if status["index_status"] != "ready":
            reasons.append("index_not_ready")
        try:
            current_model = self.store.model(doc["embedding_model_id"]) if doc else None
            if not current_model or model_descriptor(current_model) != value["embedding_provenance"]:
                reasons.append("embedding_model_provenance_unverified")
            if any(c["content_revision"] != 0 for c in text_chunks):
                reasons.append("indexed_chunk_content_changed")
            for chunk in text_chunks:
                if not chunk["embedding"] or not chunk["is_enabled"] or not chunk["vector_enabled"]:
                    reasons.append("incomplete_vector_coverage")
                    continue
                vec = json.loads(chunk["embedding"])
                if len(vec) != provenance["dimension"] or chunk["dimension"] != provenance["dimension"]:
                    reasons.append("embedding_dimension_mismatch")
                vectors.append(vec)
            embedding = aggregate(vectors) if not reasons else None
        except Exception:
            reasons.append("vector_read_or_aggregation_failed")
            embedding = None
        provenance["verified"] = embedding is not None
        result = {**status, "title": value["title"], "text": value["content"], "embedding": embedding,
                  "embedding_provenance": provenance, "chunking": value["chunking"], "aggregation": AGGREGATION,
                  "status": "ready" if embedding else "unavailable", "degraded_reasons": sorted(set(reasons))}
        if include_chunks:
            result["chunks"] = [{"chunk_id": c["id"], "knowledge_id": row["knowledge_id"], "identity": identity(value),
                "chunk_index": c["chunk_index"], "text": c["content"], "start_at": c["start_at"], "end_at": c["end_at"],
                "chunk_type": c["chunk_type"], "is_enabled": c["is_enabled"], "dimension": c["dimension"],
                "embedded_text_sha256": digest(c["embedded_text"].encode()) if c.get("embedded_text") is not None else None,
                "source_binding": "knowledge_foreign_key"} for c in chunks]
        return result

    def features(self, req):
        self.authorize(req.knowledge_base_id, req.source_scope)
        for member in req.members:
            self.authorize(req.knowledge_base_id, req.source_scope, member.model_dump())
        features, excluded, seen = [], [], set()
        total_text_bytes, total_chunks = 0, 0
        for member in req.members:
            value = member.model_dump(exclude_none=True)
            pid = publication_id({**value, "knowledge_base_id": req.knowledge_base_id})
            if pid in seen:
                raise HTTPException(400, "duplicate fixed member")
            seen.add(pid)
            row = self.journal.get(pid)
            reason = None
            if row is None:
                reason = "fixed_version_not_published"
            elif identity(row["payload"]) != identity(value) or row["payload"]["source_scope"] != req.source_scope:
                reason = "fixed_version_identity_mismatch"
            elif member.knowledge_id and member.knowledge_id != row["knowledge_id"]:
                reason = "knowledge_id_mismatch"
            elif row["source_status"] != "active":
                reason = "source_disabled"
            if reason:
                excluded.append({"identity": identity(value), "reason": reason})
                continue
            self.row(pid)
            doc, inspected_chunks, reasons = self.inspect(row)
            if not doc or {"source_disabled", "source_binding_mismatch", "index_text_mismatch"}.intersection(reasons):
                excluded.append({"identity": identity(value), "reason": reasons[0] if reasons else "upstream_unavailable"})
                continue
            total_text_bytes += len(row["payload"]["content"].encode())
            total_chunks += len(inspected_chunks)
            if total_text_bytes > 10000000 or total_chunks > 10000:
                raise HTTPException(400, "batch exceeds 10 MB text or 10000 chunks; split the fixed member list")
            features.append(self.feature(row, req.include_chunks))
        members = sorted([identity(m.model_dump()) for m in req.members], key=lambda m: json.dumps(m,sort_keys=True))
        return {"contract_version": CONTRACT, "scope_fingerprint": digest([req.knowledge_base_id, req.source_scope, members]),
                "requested_count": len(req.members), "returned_count": len(features), "features": features, "excluded": excluded,
                "population": "explicit_members_only", "statistical_unit": "document_version", "aggregation": AGGREGATION}

    def disable(self, pid, req):
        with self.write_lock:
            row = self.row(pid, receipt_only=True)
            if not row["knowledge_id"]:
                row = self.recover(row)
            self.journal.update(pid, source_status=req.state, disable_reason=req.reason)
            row = self.row(pid, receipt_only=True)
            self.ensure_binding(row)
            doc = self.get_document(row["knowledge_id"])
            if doc["parse_status"] in ("pending", "processing", "finalizing"):
                self.request("POST", "/api/v1/knowledge/" + row["knowledge_id"] + "/cancel-parse", json={})
            _, chunks, _ = self.inspect(row)
            for chunk in chunks:
                if chunk["is_enabled"] or chunk["vector_enabled"]:
                    self.request("PUT", "/api/v1/chunks/" + row["knowledge_id"] + "/" + chunk["id"], json={"is_enabled": False})
            return self.status(pid)

    def model_call(self, kind, payload):
        model = self.check_model(kind)
        params = model["parameters"]
        if params.get("provider") not in ("generic", "openai", ""):
            raise HTTPException(503, "model provider protocol is unsupported by this bounded extension")
        endpoint = params["base_url"].rstrip("/") + ("/embeddings" if kind == "embedding" else "/rerank")
        headers = {"Content-Type": "application/json"}
        if params.get("api_key"):
            headers["Authorization"] = "Bearer " + params["api_key"]
        try:
            response = self.client.post(endpoint, json={"model": model["name"], **payload}, headers=headers)
            response.raise_for_status()
            result = response.json()
        except (httpx.HTTPError, ValueError):
            raise HTTPException(502, kind + " model request failed") from None
        return result, model_descriptor(model)

    def search(self, req):
        self.authorize(req.knowledge_base_id, req.source_scope)
        rows = {}
        for pid in self.journal.all():
            row = self.journal.get(pid)
            value = row["payload"]
            if row["knowledge_id"] and row["source_status"] == "active" and value["knowledge_base_id"] == req.knowledge_base_id and value["source_scope"] == req.source_scope:
                try:
                    self.authorize(req.knowledge_base_id, req.source_scope, value)
                except HTTPException:
                    continue
                rows[row["knowledge_id"]] = row
        if len(rows) > 1000:
            raise HTTPException(400, "search scope exceeds 1000 published documents; narrow server scope")
        execution = {"keyword": "not_run", "vector": "not_run", "embedding": "not_run", "rerank": "not_run", "degraded_reasons": []}
        if not rows:
            return {"contract_version": CONTRACT, "results": [], "execution": {**execution, "degraded": False, "scope_document_count": 0}}
        query_embedding = None
        try:
            embedded, descriptor = self.model_call("embedding", {"input": [req.query_text]})
            query_embedding = embedded["data"][0]["embedding"]
            if len(query_embedding) != descriptor["dimension"] or any(not math.isfinite(x) for x in query_embedding):
                raise ValueError("bad embedding")
            execution.update(embedding="succeeded", embedding_model=descriptor,
                             embedding_usage=embedded.get("usage"), embedding_usage_status="reported" if embedded.get("usage") else "unknown")
        except (HTTPException, KeyError, IndexError, ValueError, TypeError):
            execution["embedding"] = "failed"
            execution["degraded_reasons"].append("query_embedding_failed")
        merged = {}
        for mode in ("keyword", "vector"):
            if mode == "vector" and query_embedding is None:
                execution[mode] = "skipped"
                continue
            body = {"query_text": req.query_text, "match_count": min(60, req.top_k * 3),
                    "knowledge_ids": list(rows), "disable_vector_match": mode == "keyword",
                    "disable_keywords_match": mode == "vector", "vector_threshold": 0, "keyword_threshold": 0,
                    "skip_context_enrichment": True}
            if mode == "vector":
                body["query_embedding"] = query_embedding
            try:
                data = self.request("POST", "/api/v1/knowledge-bases/" + req.knowledge_base_id + "/hybrid-search", json=body)["data"]
                execution[mode] = "succeeded"
                execution[mode + "_result_count"] = len(data)
                for rank, hit in enumerate(data):
                    kid, cid = hit.get("knowledge_id"), hit.get("id") or hit.get("chunk_id")
                    if kid not in rows or not cid:
                        continue
                    if cid not in merged:
                        merged[cid] = {**hit, "chunk_id": cid, "score": 0.0, "retrieval_modes": []}
                    merged[cid]["score"] += 1.0 / (60 + rank + 1)
                    merged[cid]["retrieval_modes"].append(mode)
            except HTTPException:
                execution[mode] = "failed"
                execution["degraded_reasons"].append(mode + "_retrieval_failed")
        if execution["keyword"] == "failed" and execution["vector"] in ("failed", "skipped"):
            raise HTTPException(502, "all retrieval channels failed")
        hits = sorted(merged.values(), key=lambda x: (-x["score"],x["chunk_id"]))
        if req.rerank and hits:
            try:
                reranked, descriptor = self.model_call("rerank", {"query": req.query_text, "documents": [h["content"] for h in hits], "top_n": len(hits)})
                scores = reranked.get("results") or reranked.get("data")
                if len(scores) != len(hits) or {s["index"] for s in scores} != set(range(len(hits))):
                    raise ValueError("incomplete rerank scores")
                for item in scores:
                    score = float(item["relevance_score"])
                    if not math.isfinite(score):
                        raise ValueError("bad rerank score")
                    hits[item["index"]]["rerank_score"] = score
                hits.sort(key=lambda h: (-h["rerank_score"],h["chunk_id"]))
                execution.update(rerank="succeeded", rerank_model=descriptor,
                                 rerank_usage=reranked.get("usage"), rerank_usage_status="reported" if reranked.get("usage") else "unknown")
            except (HTTPException, KeyError, TypeError, ValueError):
                execution["rerank"] = "failed"
                execution["degraded_reasons"].append("rerank_failed_using_reciprocal_rank_fusion")
        results = []
        statuses = {}
        for hit in hits:
            row = rows[hit["knowledge_id"]]
            value = row["payload"]
            if row["id"] not in statuses:
                statuses[row["id"]] = self.status(row["id"])
            status = statuses[row["id"]]
            if status["index_status"] != "ready":
                continue
            results.append({**hit, "publication_id": row["id"], "identity": identity(value),
                            "raw_locator": value["raw_locator"], "source_scope": value["source_scope"],
                            "content_sha256": value["content_sha256"], "index_version": status["index_version"]})
            if len(results) >= req.top_k:
                break
        execution.update(degraded=bool(execution["degraded_reasons"]), scope_document_count=len(rows),
                         index_version=digest(sorted((pid,s["index_version"]) for pid,s in statuses.items())),
                         fusion="reciprocal_rank_fusion_k60", upstream_source_commit=self.cfg.get("upstream_source_commit"),
                         upstream_image=self.cfg.get("upstream_image"))
        return {"contract_version": CONTRACT, "results": results, "execution": execution}
