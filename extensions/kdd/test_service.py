import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from fastapi import HTTPException
from fastapi.testclient import TestClient
from .app import create_app
from .models import Publication, Features, Search, Disable, Rerank
from .service import Service, UpstreamError, identity, aggregate

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
        self.rejected = 0
        self.reject_next = False
        self.fail_after_upload = False
        self.fail_disable = False
        self.s.request = self.upstream

    def upstream(self,method,path,**kwargs):
        if path == "/api/v1/knowledge-bases/"+KB:
            return {"data":{"embedding_model_id":MODEL["id"],"chunking_config":{"chunk_size":512,"chunk_overlap":80}}}
        if path.endswith("/knowledge/file"):
            meta=json.loads(kwargs["data"]["metadata"])
            # WeKnora binds metadata to map[string]string: anything else is refused, nothing is created
            if self.reject_next or any(not isinstance(v,str) for v in meta.values()):
                self.reject_next=False
                self.rejected += 1
                raise UpstreamError("WeKnora returned HTTP 400",400,"code1000 Invalid metadata format")
            self.uploads += 1
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
                self.store.docs[kid]["title"]=kwargs["json"]["title"]
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

    def test_demo_never_publishes_or_searches_a_real_scope(self):
        with self.assertRaises(HTTPException) as error:
            self.s.publish(Publication(**payload(is_demo=True)))
        self.assertEqual(error.exception.status_code, 403)
        self.assertEqual(self.uploads, 0)
        with self.assertRaises(HTTPException):
            self.s.search(Search(knowledge_base_id=KB, source_scopes=["pipeline"], query_text="trial", is_demo=True))

    def test_explicit_demo_scope_roundtrip_and_real_exclusion(self):
        self.cfg["demo_kb_ids"] = [KB]
        self.path.write_text(json.dumps(self.cfg))
        with self.assertRaises(HTTPException):
            self.s.publish(Publication(**payload()))
        receipt = self.s.publish(Publication(**payload(is_demo=True)))
        self.assertTrue(receipt["is_demo"])
        self.assertEqual(self.store.docs[receipt["knowledge_id"]]["metadata"]["is_demo"], "true")
        request = self.member_request().model_copy(update={"is_demo": True})
        features = self.s.features(request)
        self.assertTrue(features["is_demo"])
        self.assertTrue(features["features"][0]["is_demo"])
        with self.assertRaises(HTTPException):
            self.s.features(self.member_request())
        with self.assertRaises(HTTPException):
            self.s.search(Search(knowledge_base_id=KB, source_scopes=["pipeline"], query_text="trial"))

    def test_reclassifying_kb_cannot_relabel_existing_publication(self):
        receipt = self.s.publish(Publication(**payload()))
        self.cfg["demo_kb_ids"] = [KB]
        self.path.write_text(json.dumps(self.cfg))
        with self.assertRaises(HTTPException) as error:
            self.s.publish(Publication(**payload(is_demo=True)))
        self.assertEqual(error.exception.status_code, 409)
        self.assertFalse(self.s.status(receipt["publication_id"])["is_demo"])
        self.assertEqual(self.uploads, 1)

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

    def test_native_parser_snapshot_and_real_length_title(self):
        original=self.s.request
        def processing_upload(method,path,**kwargs):
            result=original(method,path,**kwargs)
            if path.endswith("/knowledge/file"):
                self.store.docs[result["data"]["id"]]["parse_status"]="processing"
            return result
        self.s.request=processing_upload
        title="T"*286  # Observed ClinicalTrials NCT04165083 briefTitle length.
        receipt=self.s.publish(Publication(**payload(title=title)))
        kid=receipt["knowledge_id"]
        self.assertEqual(receipt["binding_status"],"pending")
        self.assertEqual(receipt["source_status"],"active")
        self.assertEqual(self.store.docs[kid]["custom_metadata"],{})
        # Native parse completion writes its original empty custom_metadata.
        self.store.docs[kid]["parse_status"]="completed"
        final=self.s.publish(Publication(**payload(title=title)))
        self.assertEqual(final["index_status"],"ready")
        self.assertEqual(self.uploads,1)
        self.assertEqual(len(self.store.docs[kid]["title"]),255)
        self.assertEqual(self.s.features(self.member_request())["features"][0]["title"],title)
        self.assertTrue(final["display_title_truncated"])
        self.store.docs[kid]["custom_metadata"]["raw_sha256"]="b"*64
        with self.assertRaises(HTTPException) as e:self.s.publish(Publication(**payload(title=title)))
        self.assertEqual(e.exception.status_code,409)
        self.assertEqual(self.store.docs[kid]["custom_metadata"]["raw_sha256"],"b"*64)

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
        self.s.publish(Publication(**payload()))
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

    def _search_upstream(self, hits, calls, fail=()):
        original=self.s.request
        def upstream(method,path,**kwargs):
            if path.endswith("/hybrid-search"):
                mode="keyword" if kwargs["json"]["disable_vector_match"] else "vector"
                calls.append((mode,kwargs["json"]))
                if mode in fail:
                    raise HTTPException(502,"down")
                return {"data":copy.deepcopy(hits) or None}
            return original(method,path,**kwargs)
        self.s.request=upstream

    def _models(self, reranks):
        def model_call(kind,body):
            if kind=="embedding":
                return {"data":[{"embedding":[1.0,0.0]}]},{"dimension":2}
            reranks.append(body)
            return {"results":[{"index":i,"relevance_score":1.0-i/100} for i in range(len(body["documents"]))]},{"model_id":"reranker"}
        self.s.model_call=model_call

    def test_whole_kb_recall_returns_only_active_authorized_hits_and_counts_the_rest(self):
        meta={"doc_type":"registry_record","identifiers":["nct:NCT1"],
              "published_at_meaning":"who_notice_publication","source_updated_at_meaning":"who_notice_update"}
        active=self.s.publish(Publication(**payload(metadata=meta)))
        gone=self.s.publish(Publication(**payload(source_record_id="NCT2",raw_sha256="b"*64,content="Second fixed text.",
            raw_locator=dict(source_instance="fabric",source_id="clinicaltrials",source_record_id="NCT2",version_id="v1"))))
        self.s.disable(gone["publication_id"],Disable(reason="test",state="revoked"))
        hits=[{"id":"c1","knowledge_id":active["knowledge_id"],"content":"Actual fixed text.","score":2.0},
              {"id":"c2","knowledge_id":gone["knowledge_id"],"content":"Second fixed text.","score":1.5},
              {"id":"c3","knowledge_id":"someone-elses-knowledge","content":"private","score":1.0}]
        calls,reranks=[],[]
        self._search_upstream(hits,calls)
        self._models(reranks)
        result=self.s.search(Search(knowledge_base_id=KB,source_scopes=["pipeline"],query_text="trial",top_k=60))
        self.assertEqual([r["chunk_id"] for r in result["results"]],["c1"])
        self.assertEqual(result["results"][0]["metadata"],meta)
        self.assertEqual(result["results"][0]["retrieval_modes"],["keyword","vector"])
        self.assertNotIn("_row",result["results"][0])
        self.assertEqual(result["execution"]["filtered_counts"],{"disabled":2,"not_registered":2})
        self.assertNotIn("private",json.dumps(result))
        # one call per channel over the whole knowledge base: no knowledge-id list, no per-hit status reads
        self.assertEqual(len(calls),2)
        self.assertTrue(all("knowledge_ids" not in body and body["match_count"]==60 for _,body in calls))
        self.assertEqual(reranks,[])  # candidate recall: no model reads unverified text

    def test_one_recall_covers_every_requested_scope_of_a_knowledge_base(self):
        """Scopes sharing a knowledge base are searched in one recall, so they do not crowd each other out."""
        self.cfg["source_mappings"][0]["scopes"]=["pipeline","research"]
        self.path.write_text(json.dumps(self.cfg))
        a=self.s.publish(Publication(**payload()))
        b=self.s.publish(Publication(**payload(source_scope="research",source_record_id="NCT9",raw_sha256="c"*64,
            content="Research fixed text.",raw_locator=dict(source_instance="fabric",source_id="clinicaltrials",
            source_record_id="NCT9",version_id="v1"))))
        hits=[{"id":"c1","knowledge_id":a["knowledge_id"],"content":"x"},{"id":"c2","knowledge_id":b["knowledge_id"],"content":"y"}]
        calls,reranks=[],[]
        self._search_upstream(hits,calls)
        self._models(reranks)
        both=self.s.search(Search(knowledge_base_id=KB,source_scopes=["pipeline","research"],query_text="t"))
        self.assertEqual(sorted(r["chunk_id"] for r in both["results"]),["c1","c2"])
        self.assertEqual(len(calls),2)  # one keyword and one vector call for both scopes
        one=self.s.search(Search(knowledge_base_id=KB,source_scopes=["research"],query_text="t"))
        self.assertEqual([r["chunk_id"] for r in one["results"]],["c2"])
        self.assertEqual(one["execution"]["filtered_counts"],{"other_scope":2})
        with self.assertRaises(HTTPException):
            self.s.search(Search(knowledge_base_id=KB,source_scopes=["pipeline","business"],query_text="t"))

    def test_search_is_recall_only_and_a_failed_channel_is_degraded(self):
        from pydantic import ValidationError
        with self.assertRaises(ValidationError):  # no blind-rerank entry left to bypass verify-then-rerank
            Search(knowledge_base_id=KB,source_scopes=["pipeline"],query_text="trial",rerank=True)
        receipt=self.s.publish(Publication(**payload()))
        calls,reranks=[],[]
        self._search_upstream([{"id":"c1","knowledge_id":receipt["knowledge_id"],"content":"Actual fixed text."}],calls,fail=("keyword",))
        self._models(reranks)
        result=self.s.search(Search(knowledge_base_id=KB,source_scopes=["pipeline"],query_text="trial"))
        self.assertEqual(result["execution"]["keyword"],"failed")
        self.assertIn("keyword_retrieval_failed",result["execution"]["degraded_reasons"])
        self.assertEqual(len(result["results"]),1)
        self.assertEqual(reranks,[])
        self._search_upstream([],calls,fail=("keyword","vector"))
        with self.assertRaises(HTTPException) as error:
            self.s.search(Search(knowledge_base_id=KB,source_scopes=["pipeline"],query_text="trial"))
        self.assertEqual(error.exception.status_code,502)

    def test_rerank_scores_caller_verified_texts_with_the_pinned_model(self):
        reranks=[]
        self._models(reranks)
        result=self.s.rerank(Rerank(knowledge_base_id=KB,source_scope="pipeline",query_text="trial",documents=["a","b"]))
        self.assertEqual([s["index"] for s in result["scores"]],[0,1])
        self.assertEqual(result["rerank_model"],{"model_id":"reranker"})
        self.assertEqual(reranks[0]["documents"],["a","b"])
        with self.assertRaises(HTTPException):
            self.s.rerank(Rerank(knowledge_base_id=KB,source_scope="other",query_text="trial",documents=["a"]))
        self.s.model_call=lambda kind,body:({"results":[{"index":0,"relevance_score":1.0}]},{"model_id":"r"})
        with self.assertRaises(HTTPException) as error:
            self.s.rerank(Rerank(knowledge_base_id=KB,source_scope="pipeline",query_text="trial",documents=["a","b"]))
        self.assertEqual(error.exception.status_code,502)

    def test_metadata_is_bounded_descriptive_and_not_identity(self):
        from pydantic import ValidationError
        with self.assertRaises(ValidationError):
            Publication(**payload(metadata={"identifiers":["x"]*51}))
        many=["doi:10.1000/synthetic.identifier.%04d"%i for i in range(50)]
        receipt=self.s.publish(Publication(**payload(metadata={"doc_type":"article","identifiers":many})))
        doc=self.store.docs[receipt["knowledge_id"]]
        compact=json.loads(doc["metadata"]["fabric_metadata"])
        self.assertEqual(compact,{"doc_type":"article"})  # identifiers dropped to fit one native value
        self.assertEqual(doc["custom_metadata"]["fabric_metadata"],doc["metadata"]["fabric_metadata"])
        # a replay with other metadata is the same publication: metadata takes no part in identity or conflicts
        again=self.s.publish(Publication(**payload(metadata={"doc_type":"other"})))
        self.assertEqual(again["publication_id"],receipt["publication_id"])
        self.assertEqual(self.uploads,1)

    def test_an_unknown_metadata_key_is_logged_and_accepted_but_identity_and_kb_are_still_checked(self):
        with patch("extensions.kdd.app.Service", return_value=self.s):
            client=TestClient(create_app(self.path))
        headers={"Authorization":"Bearer private-token"}
        meta={"doc_type":"article","brand_new_key":"x"}
        def post(**changes):
            return client.post("/api/v1/kdd/publications",headers=headers,json=payload(metadata=meta,**changes))
        with self.assertLogs("extensions.kdd.models",level="WARNING") as logs:
            # before this change the unknown key made the whole publication an HTTP 400
            first=post()
            self.assertEqual(first.status_code,200)
            # a retry is the same publication: nothing is written twice
            self.assertEqual(post().status_code,200)
            self.assertEqual(self.uploads,1)
            # the key is kept with the other descriptive metadata
            stored=self.store.docs[first.json()["knowledge_id"]]["metadata"]["fabric_metadata"]
            self.assertEqual(json.loads(stored),meta)
            # the same request still fails when the identity or the knowledge base does not match
            wrong_locator=payload(metadata=meta)
            wrong_locator["raw_locator"]["source_record_id"]="NCT2"
            self.assertEqual(client.post("/api/v1/kdd/publications",headers=headers,json=wrong_locator).status_code,400)
            self.assertEqual(post(knowledge_base_id="another-kb").status_code,403)
            self.assertEqual(self.uploads,1)
        self.assertIn("brand_new_key",logs.output[0])

    def test_pieces_of_a_long_artifact_are_separate_publications_with_their_identity(self):
        import hashlib
        text="Actual fixed text."
        def piece(index, content):
            return {"rule":"datafabric.index_piece.v1","index":index,"count":2,"start":0 if index==1 else 10,
                    "end":10 if index==1 else 18,"context_chars":len(content)-(10 if index==1 else 8),
                    "chars":len(content),"bytes":len(content.encode()),"sha256":hashlib.sha256(content.encode()).hexdigest()}
        whole=self.s.publish(Publication(**payload(artifact_key="proc.v1:"+"a"*64)))
        first=self.s.publish(Publication(**payload(artifact_key="proc.v1:"+"a"*64,content=text[:10],piece=piece(1,text[:10]))))
        second=self.s.publish(Publication(**payload(artifact_key="proc.v1:"+"a"*64,content=text[10:],piece=piece(2,text[10:]))))
        self.assertEqual(len({whole["publication_id"],first["publication_id"],second["publication_id"]}),3)
        # Codex 456/460: same index under another rule or another range is another publication
        from .service import publication_id
        base=payload(artifact_key="proc.v1:"+"a"*64,content=text[:10],piece=piece(1,text[:10]))
        ids={publication_id(dict(base,piece=dict(base["piece"],**change))) for change in
             ({},{"rule":"datafabric.index_piece.v2"},{"end":12},{"sha256":"e"*64})}
        self.assertEqual(len(ids),4)
        self.assertIn(first["publication_id"],ids)
        self.assertEqual(first["identity"]["piece"]["index"],1)
        self.assertNotIn("piece",whole["identity"])  # 不分片：身份与原来一致
        with self.assertRaises(HTTPException) as error:
            self.s.publish(Publication(**payload(artifact_key="proc.v1:"+"a"*64,content="other",piece=piece(2,text[10:]))))
        self.assertEqual(error.exception.status_code,400)

    def piece_payload(self, **changes):
        import hashlib
        text="Actual fixed text."
        piece={"rule":"datafabric.index_piece.v1","index":1,"count":2,"start":0,"end":10,"context_chars":0,
               "chars":10,"bytes":10,"sha256":hashlib.sha256(text[:10].encode()).hexdigest()}
        return payload(artifact_key="proc.v1:"+"a"*64,content=text[:10],piece=piece,**changes)

    def test_piece_goes_on_the_wire_as_one_canonical_string(self):
        receipt=self.s.publish(Publication(**self.piece_payload()))
        self.assertEqual((self.uploads,self.rejected),(1,0))
        meta=self.store.docs[receipt["knowledge_id"]]["metadata"]
        self.assertTrue(all(isinstance(v,str) for v in meta.values()))
        self.assertEqual(json.loads(meta["piece"]),receipt["identity"]["piece"])
        self.assertEqual(meta["piece"],json.dumps(receipt["identity"]["piece"],sort_keys=True,ensure_ascii=False,separators=(",",":")))
        self.assertIsInstance(receipt["identity"]["piece"],dict)  # the structured identity is unchanged
        self.assertEqual(receipt["binding_status"],"bound")
        self.assertEqual([a["outcome"] for a in self.s.journal.attempts(receipt["publication_id"])],["uploaded"])

    def test_tampered_piece_or_artifact_string_is_a_binding_mismatch(self):
        for field,forged in (("piece",lambda v:v.replace('"end":10','"end":11')),("artifact_key",lambda v:v[:-1]+"b")):
            with self.subTest(field=field):
                receipt=self.s.publish(Publication(**self.piece_payload(version_id="v-"+field)))
                pid,kid=receipt["publication_id"],receipt["knowledge_id"]
                doc=self.store.docs[kid]
                doc["metadata"][field]=forged(doc["metadata"][field])
                with self.assertRaises(HTTPException) as e:self.s.publish(Publication(**self.piece_payload(version_id="v-"+field)))
                self.assertEqual(e.exception.status_code,409)
                self.assertIn("source_binding_mismatch",self.s.inspect(self.s.journal.get(pid))[2])
                self.store.docs.pop(kid)

    def test_first_explicit_rejection_is_kept_and_only_it_may_be_uploaded_again(self):
        self.reject_next=True
        with self.assertRaises(HTTPException):self.s.publish(Publication(**payload()))
        (pid,)=self.s.journal.all()
        self.assertEqual(self.s.journal.get(pid)["upload_status"],"rejected")
        first=self.s.journal.attempts(pid)[0]
        self.assertEqual((first["outcome"],first["http_status"],first["reason"]),("rejected",400,"code1000 Invalid metadata format"))
        receipt=self.s.publish(Publication(**payload()))  # nothing was created upstream: a second upload is safe
        self.assertEqual((self.uploads,self.rejected,receipt["binding_status"]),(1,1,"bound"))
        self.assertEqual([(a["attempt"],a["outcome"],a["http_status"]) for a in self.s.journal.attempts(pid)],
                         [(1,"rejected",400),(2,"uploaded",None)])

    def test_unknown_outcome_is_recorded_and_never_overwritten_by_the_recover_409(self):
        original=self.s.request
        def timeout(method,path,**kwargs):
            if path.endswith("/knowledge/file"):
                raise UpstreamError("WeKnora request outcome unavailable",None,"transport")
            return original(method,path,**kwargs)
        self.s.request=timeout
        with self.assertRaises(HTTPException):self.s.publish(Publication(**payload()))
        self.s.request=original
        (pid,)=self.s.journal.all()
        for _ in range(2):
            with self.assertRaises(HTTPException) as e:self.s.publish(Publication(**payload()))
            self.assertEqual(e.exception.status_code,409)
        self.assertEqual(self.uploads,0)
        self.assertEqual(self.s.journal.get(pid)["upload_status"],"unknown")
        self.assertEqual([(a["outcome"],a["http_status"],a["reason"]) for a in self.s.journal.attempts(pid)],
                         [("unknown",None,"transport")])

    def test_lost_retry_stays_in_flight_and_is_only_reconciled(self):
        self.reject_next=True
        with self.assertRaises(HTTPException):self.s.publish(Publication(**payload()))
        (pid,)=self.s.journal.all()
        original=self.s.request
        def lost(method,path,**kwargs):
            result=original(method,path,**kwargs)
            if path.endswith("/knowledge/file"):raise SystemExit("process lost after the upstream accepted")
            return result
        self.s.request=lost
        with self.assertRaises(SystemExit):self.s.publish(Publication(**payload()))
        self.s.request=original
        self.assertEqual(self.s.journal.get(pid)["upload_status"],"sending")  # the earlier rejection no longer applies
        receipt=self.s.publish(Publication(**payload()))
        self.assertEqual((self.uploads,receipt["binding_status"]),(1,"bound"))
        self.assertEqual([(a["attempt"],a["outcome"]) for a in self.s.journal.attempts(pid)],[(1,"rejected"),(2,"in_flight")])

    def test_receipt_names_the_latest_upload_attempt(self):
        self.reject_next=True
        with self.assertRaises(HTTPException):self.s.publish(Publication(**payload()))
        (pid,)=self.s.journal.all()
        self.assertEqual(self.s.status(pid)["last_attempt"],{"attempt":1,"outcome":"rejected"})
        receipt=self.s.publish(Publication(**payload()))
        self.assertEqual(receipt["last_attempt"],{"attempt":2,"outcome":"uploaded"})

    def test_request_types_the_upstream_outcome_without_content(self):
        import httpx
        class Client:
            def __init__(self,outcome):self.outcome=outcome
            def request(self,method,url,**kwargs):
                if isinstance(self.outcome,Exception):raise self.outcome
                return self.outcome
        cases=((httpx.Response(400,json={"code":1000,"message":"Invalid metadata format"}),400,"code1000 Invalid metadata format"),
               (httpx.Response(503,text="<html>busy</html>"),503,"http_503"),
               (httpx.ReadTimeout("slow"),None,"transport"))
        for outcome,status,reason in cases:
            with self.subTest(status=status):
                self.s.client=Client(outcome)
                with self.assertRaises(UpstreamError) as e:Service.request(self.s,"POST","/api/v1/x")
                self.assertEqual((e.exception.status_code,e.exception.upstream_status,e.exception.reason),(502,status,reason))
                self.assertNotIn("private-key",str(e.exception.detail)+e.exception.reason)

    def test_more_chunks_than_verified_is_a_budget_limit_not_an_outage(self):
        receipt=self.s.publish(Publication(**payload()))
        def too_many(kb,kid):
            raise ValueError("document exceeds bounded chunk limit")
        self.store.document=too_many
        status=self.s.status(receipt["publication_id"])
        self.assertEqual(status["index_status"],"budget_exceeded")
        self.assertIn("chunk_budget_exceeded",status["degraded_reasons"])

    def test_a_vector_store_failure_keeps_the_parse_status_the_upstream_api_confirmed(self):
        # Codex r89: a slow vector check failed and the status said parse_status=unknown although the
        # knowledge was completed upstream; the confirmed status stays, and nothing is ready
        receipt=self.s.publish(Publication(**payload()))
        confirmed=self.s.get_document(receipt["knowledge_id"])["parse_status"]
        def broken(kb,kid):
            raise RuntimeError("canceling statement due to statement timeout")
        self.store.document=broken
        status=self.s.status(receipt["publication_id"])
        self.assertEqual((status["vector_status"],status["index_status"]),("unavailable","unavailable"))
        self.assertEqual(status["parse_status"],confirmed)
        self.assertNotEqual(status["parse_status"],"unknown")
        self.assertIn("vector_store_unavailable",status["degraded_reasons"])

    def test_aggregation_normalizes_chunks_and_rejects_bad_vectors(self):
        self.assertEqual(aggregate([[3,0],[6,0]]),[1.0,0.0])
        with self.assertRaises(ValueError):aggregate([[1,0],[1]])
        with self.assertRaises(ValueError):aggregate([[float("nan"),0]])

if __name__=="__main__":unittest.main()
