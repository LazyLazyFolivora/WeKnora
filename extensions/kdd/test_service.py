import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import httpx
from fastapi import HTTPException
from fastapi.testclient import TestClient
from .app import create_app
from .models import Publication, Features, Search, Disable
from .service import Service, identity, publication_id, digest, model_descriptor, aggregate

KB = "bounded-kb"
MODEL = {"id": "builtin-embedding-default", "name": "bge-m3", "source": "remote", "updated_at": "2026-10-30T00:00:00Z",
         "parameters": {"provider": "generic", "base_url": "http://model.invalid/v1", "embedding_parameters": {"dimension": 2}}}

def payload(**changes):
    v = dict(source_instance="fabric",source_id="clinicaltrials",source_record_id="NCT1",version_id="v1",raw_sha256="a"*64,
             knowledge_base_id=KB,source_scope="pipeline",title="Clinical trial",content="Actual fixed text.",
             raw_locator=dict(source_instance="fabric",source_id="clinicaltrials",source_record_id="NCT1",version_id="v1"))
    v.update(changes)
    if "version_id" in changes:
        v["raw_locator"]["version_id"] = changes["version_id"]
    return v

class FakeStore:
    def __init__(self):
        self.docs = {}
        self.chunks = {}
        self.current_model = copy.deepcopy(MODEL)
    def model(self, mid):
        return self.current_model if self.current_model and mid == self.current_model["id"] else None
    def document(self,kb,kid):
        d = self.docs.get(kid)
        return (copy.deepcopy(d),copy.deepcopy(self.chunks.get(kid,[]))) if d and d["knowledge_base_id"] == kb else (None,[])

class ServiceTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name)/"config.json"
        self.cfg = dict(token="private-token",journal_path=str(Path(self.tmp.name)/"state.sqlite"),db={},models=[MODEL],
                        upstream_url="http://upstream.invalid",upstream_api_key="private-key",allowed_kb_ids=[KB],
                        source_mappings=[dict(source_instance="fabric",source_id="clinicaltrials",knowledge_base_id=KB,scopes=["pipeline"])])
        self.path.write_text(json.dumps(self.cfg))
        self.store = FakeStore()
        self.s = Service(self.path,store=self.store)
        self.uploads = 0
        self.fail_after_upload = False
        self.fail_disable = False
        self.s.request = self.upstream

    def upstream(self,method,path,**kwargs):
        if path == "/api/v1/knowledge-bases/"+KB:
            return {"data":{"embedding_model_id":MODEL["id"],"chunking_config":{"chunk_size":512,"chunk_overlap":80}}}
        if path.endswith("/knowledge/file"):
            self.uploads += 1
            meta=json.loads(kwargs["data"]["metadata"])
            kid="knowledge-"+str(self.uploads)
            self.store.docs[kid]=dict(id=kid,knowledge_base_id=KB,tenant_id=10000,embedding_model_id=MODEL["id"],
                parse_status="completed",enable_status="enabled",metadata=meta,custom_metadata={},processed_at="now",updated_at="now")
            self.store.chunks[kid]=[dict(id="chunk-"+kid,content="Actual fixed text.",chunk_index=0,chunk_type="text",is_enabled=True,
                start_at=0,end_at=18,content_revision=0,index_status="indexed",updated_at="now",dimension=2,embedding="[1,0]",
                vector_enabled=True,vector_updated_at="now")]
            if self.fail_after_upload:
                self.fail_after_upload=False
                raise HTTPException(502,"unknown outcome")
            return {"data":{"id":kid}}
        if path.endswith("/knowledge"):
            return {"data":list(self.store.docs.values())}
        if path.startswith("/api/v1/knowledge/"):
            kid=path.split("/")[4]
            if path.endswith("/cancel-parse"):
                self.store.docs[kid]["parse_status"]="cancelled"
            if method=="PUT":
                self.store.docs[kid]["custom_metadata"]=kwargs["json"]["custom_metadata"]
            return {"data":copy.deepcopy(self.store.docs[kid])}
        if path.startswith("/api/v1/chunks/"):
            if self.fail_disable:
                raise HTTPException(502,"disable failed")
            kid=path.split("/")[4]
            for c in self.store.chunks[kid]:
                c["is_enabled"]=False;c["vector_enabled"]=False
            return {"data":{}}
        raise AssertionError((method,path))

    def member_request(self, value=None):
        v = value or payload()
        return Features(knowledge_base_id=KB,source_scope="pipeline",members=[identity(v)])

    def test_idempotent_versions_and_hash_conflicts(self):
        first=self.s.publish(Publication(**payload()))
        again=self.s.publish(Publication(**payload()))
        self.assertEqual(first["knowledge_id"],again["knowledge_id"])
        self.assertEqual(self.uploads,1)
        with self.assertRaises(HTTPException) as e:
            self.s.publish(Publication(**payload(content="changed")))
        self.assertEqual(e.exception.status_code,409)
        second=self.s.publish(Publication(**payload(version_id="v2")))
        self.assertNotEqual(first["knowledge_id"],second["knowledge_id"])
        self.assertNotEqual(first["indexed_text_sha256"],second["indexed_text_sha256"])
        self.assertEqual(first["content_sha256"],second["content_sha256"])

    def test_unknown_upload_reconciles_without_resending(self):
        self.fail_after_upload=True
        with self.assertRaises(HTTPException):
            self.s.publish(Publication(**payload()))
        receipt=self.s.publish(Publication(**payload()))
        self.assertEqual(self.uploads,1)
        self.assertEqual(receipt["binding_status"],"bound")

    def test_unknown_missing_receipt_never_blindly_resends(self):
        original=self.s.request
        def fail(method,path,**kwargs):
            if path.endswith("/knowledge/file"):
                raise HTTPException(502,"request outcome unknown")
            return original(method,path,**kwargs)
        self.s.request=fail
        with self.assertRaises(HTTPException):self.s.publish(Publication(**payload()))
        self.s.request=original
        with self.assertRaises(HTTPException) as e:self.s.publish(Publication(**payload()))
        self.assertEqual(e.exception.status_code,409)
        self.assertEqual(self.uploads,0)

    def test_scope_is_not_authorization(self):
        with self.assertRaises(HTTPException) as e:
            self.s.publish(Publication(**payload(source_id="foreign")))
        self.assertEqual(e.exception.status_code,403)
        self.assertEqual(self.uploads,0)
        with self.assertRaises(HTTPException):
            self.s.publish(Publication(**payload(knowledge_base_id="another-kb")))
        with self.assertRaises(HTTPException):
            self.s.publish(Publication(**payload(source_scope="public_health")))

    def test_explicit_fixed_members_and_vector_provenance(self):
        receipt=self.s.publish(Publication(**payload()))
        result=self.s.features(self.member_request())
        self.assertEqual(result["population"],"explicit_members_only")
        self.assertEqual(result["features"][0]["embedding"],[1.0,0.0])
        self.assertTrue(result["features"][0]["embedding_provenance"]["verified"])
        self.store.current_model=None
        result=self.s.features(self.member_request())
        self.assertIsNone(result["features"][0]["embedding"])
        self.assertIn("embedding_model_provenance_unverified",result["features"][0]["degraded_reasons"])
        self.assertEqual(result["features"][0]["text"],payload()["content"])

    def test_completed_parse_is_not_vector_ready(self):
        receipt=self.s.publish(Publication(**payload()))
        self.store.chunks[receipt["knowledge_id"]][0]["embedding"]=None
        self.assertEqual(self.s.status(receipt["publication_id"])["vector_status"],"missing")
        self.assertIsNone(self.s.features(self.member_request())["features"][0]["embedding"])

    def test_edited_chunks_and_wrong_raw_hash_do_not_get_features(self):
        receipt=self.s.publish(Publication(**payload()))
        self.store.chunks[receipt["knowledge_id"]][0]["content_revision"]=1
        f=self.s.features(self.member_request())["features"][0]
        self.assertIsNone(f["embedding"])
        mismatch=self.s.features(self.member_request(payload(raw_sha256="b"*64)))
        self.assertEqual(mismatch["returned_count"],0)
        self.assertEqual(mismatch["excluded"][0]["reason"],"fixed_version_identity_mismatch")

    def test_disable_excludes_before_upstream_completion_and_can_resume(self):
        receipt=self.s.publish(Publication(**payload()))
        self.fail_disable=True
        with self.assertRaises(HTTPException):
            self.s.disable(receipt["publication_id"],Disable(reason="withdrawn",state="withdrawn"))
        self.assertEqual(self.s.features(self.member_request())["returned_count"],0)
        self.fail_disable=False
        final=self.s.disable(receipt["publication_id"],Disable(reason="withdrawn",state="withdrawn"))
        self.assertEqual(final["index_status"],"disabled")
        self.assertEqual(final["vector_count"],0)

    def test_permission_removal_is_immediate(self):
        receipt=self.s.publish(Publication(**payload()))
        self.cfg["source_mappings"]=[]
        self.path.write_text(json.dumps(self.cfg))
        with self.assertRaises(HTTPException) as e:self.s.features(self.member_request())
        self.assertEqual(e.exception.status_code,403)
        status=self.s.status(receipt["publication_id"])
        self.assertNotIn("content",status)
        final=self.s.disable(receipt["publication_id"],Disable(reason="permission revoked",state="revoked"))
        self.assertEqual(final["index_status"],"disabled")

    def test_hash_validation_and_duplicate_members(self):
        with self.assertRaises(HTTPException) as e:self.s.publish(Publication(**payload(content_sha256="c"*64)))
        self.assertEqual(e.exception.status_code,400)
        self.s.publish(Publication(**payload()))
        request=self.member_request();request.members*=2
        with self.assertRaises(HTTPException):self.s.features(request)

    def test_http_auth_and_invalid_body_do_not_echo_text(self):
        with patch("extensions.kdd.app.Service", return_value=self.s):
            client=TestClient(create_app(self.path))
        self.assertEqual(client.post("/api/v1/kdd/features",json={}).status_code,401)
        response=client.post("/api/v1/kdd/features",headers={"Authorization":"Bearer private-token"},json={"secret":"private document"})
        self.assertEqual(response.status_code,400)
        self.assertNotIn("private document",response.text)

    def test_search_index_read_failure_is_degraded_not_complete_no_hits(self):
        receipt=self.s.publish(Publication(**payload()))
        original=self.s.request
        def upstream(method,path,**kwargs):
            if path.endswith("/hybrid-search"):
                return {"data":[{"id":"chunk-one","knowledge_id":receipt["knowledge_id"],"content":"matched text"}]}
            return original(method,path,**kwargs)
        self.s.request=upstream
        def model_call(kind,body):
            if kind=="embedding":return {"data":[{"embedding":[1.0,0.0]}]}, {"dimension":2}
            return {"results":[{"index":0,"relevance_score":0.9}]}, {"model_id":"reranker"}
        self.s.model_call=model_call
        self.s.status=lambda pid:{"index_status":"unavailable","degraded_reasons":["vector_store_unavailable"],"index_version":"unknown"}
        result=self.s.search(Search(knowledge_base_id=KB,source_scope="pipeline",query_text="trial"))
        self.assertEqual(result["results"],[])
        self.assertTrue(result["execution"]["degraded"])
        self.assertIn("vector_store_unavailable",result["execution"]["degraded_reasons"])
        self.assertEqual(result["execution"]["coverage_status"],"verification_incomplete")
        self.assertEqual(result["execution"]["filtered_publications"][0]["reason_type"],"unverified")
        self.s.status=lambda pid:{"index_status":"disabled","source_status":"disabled","degraded_reasons":["source_disabled"],"index_version":"disabled"}
        result=self.s.search(Search(knowledge_base_id=KB,source_scope="pipeline",query_text="trial"))
        self.assertEqual(result["execution"]["filtered_publications"][0]["reason_type"],"disabled")
        self.assertFalse(result["execution"]["degraded"])

    def test_aggregation_normalizes_chunks_and_rejects_bad_vectors(self):
        self.assertEqual(aggregate([[3,0],[6,0]]),[1.0,0.0])
        with self.assertRaises(ValueError):aggregate([[1,0],[1]])
        with self.assertRaises(ValueError):aggregate([[float("nan"),0]])

if __name__=="__main__":unittest.main()
