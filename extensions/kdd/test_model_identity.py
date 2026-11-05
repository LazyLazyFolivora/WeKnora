import copy
import json
import unittest

import httpx
from fastapi import HTTPException
from .models import Publication, Search, Rerank
from .service import model_descriptor
from .test_service import ServiceTests, MODEL, KB, payload


class ModelIdentityTests(unittest.TestCase):
    setUp = ServiceTests.setUp
    upstream = ServiceTests.upstream
    member_request = ServiceTests.member_request

    def test_timestamp_only_restart_preserves_pinned_model_and_existing_features(self):
        receipt = self.s.publish(Publication(**payload()))
        before = self.s.journal.get(receipt['publication_id'])['payload']['embedding_provenance']
        self.store.current_model['updated_at'] = '2026-11-01T12:00:00Z'
        self.store.current_model['description'] = 'operator note'
        pinned = self.s.check_model('embedding')
        self.assertEqual(model_descriptor(pinned), before)
        result = self.s.features(self.member_request())['features'][0]
        self.assertTrue(result['embedding_provenance']['verified'])
        self.assertEqual(result['embedding'], [1.0, 0.0])
        self.assertEqual(self.s.journal.get(receipt['publication_id'])['payload']['embedding_provenance'], before)
        self.assertEqual(self.uploads, 1)
        self.assertIsNone(result['embedding_provenance']['artifact_version'])

    def test_semantic_changes_reject_before_provider_and_invalidate_features(self):
        self.s.publish(Publication(**payload()))
        changes = [
            ('name', 'other-model'), ('type', 'Rerank'), ('source', 'local'),
            ('parameters.provider', 'openai'), ('parameters.base_url', 'http://other.invalid/v1'),
            ('parameters.interface_type', 'other-interface'),
            ('parameters.embedding_parameters.dimension', 3),
            ('parameters.embedding_parameters.truncate_prompt_tokens', 512),
            ('parameters.embedding_parameters.supports_dimension_override', True),
            ('parameters.extra_config', {'api_version': '2026-11-01'}),
            ('parameters.custom_headers', {'X-Model-Route': 'other-deployment'}),
        ]
        provider_requests = []
        def unexpected(request):
            provider_requests.append(request)
            return httpx.Response(200, json={'data': [{'embedding': [1, 0]}]})
        self.s.client = httpx.Client(transport=httpx.MockTransport(unexpected))
        self.addCleanup(self.s.client.close)
        for path, value in changes:
            with self.subTest(field=path):
                current = copy.deepcopy(MODEL)
                target = current
                keys = path.split('.')
                for key in keys[:-1]:
                    target = target.setdefault(key, {})
                target[keys[-1]] = value
                self.store.current_model = current
                with self.assertRaises(HTTPException) as failure:
                    self.s.model_call('embedding', {'input': ['local fixture']})
                self.assertEqual(failure.exception.status_code, 409)
                feature = self.s.features(self.member_request())['features'][0]
                self.assertFalse(feature['embedding_provenance']['verified'])
                self.assertIsNone(feature['embedding'])
        self.assertEqual(provider_requests, [])

    def test_credentials_and_equivalent_endpoint_do_not_reidentify_or_leak(self):
        pinned = copy.deepcopy(MODEL)
        pinned['parameters'].update(api_key='pinned-secret', custom_headers={'Authorization': 'header-secret'})
        self.s.cfg['models'] = [pinned]
        current = copy.deepcopy(pinned)
        current['updated_at'] = '2026-11-01T12:00:00Z'
        current['parameters'].update(api_key='rotated-secret', base_url='http://MODEL.invalid:80/v1/',
                                     custom_headers={'authorization': 'rotated-header'})
        current['parameters']['embedding_parameters'].update(truncate_prompt_tokens=0, supports_dimension_override=False)
        self.store.current_model = current
        descriptor = model_descriptor(self.s.check_model('embedding'))
        self.assertEqual(descriptor, model_descriptor(pinned))
        self.assertIsNone(descriptor['artifact_version'])
        for secret in ['pinned-secret', 'rotated-secret', 'header-secret', 'rotated-header']:
            self.assertNotIn(secret, json.dumps(descriptor))

    def test_search_retains_safe_model_configuration_failure_codes(self):
        receipt = self.s.publish(Publication(**payload()))
        rerank = copy.deepcopy(MODEL)
        rerank.update(id='builtin-rerank-default', name='reranker', type='Rerank')
        self.s.cfg['models'].append(rerank)
        current_models = {m['id']: copy.deepcopy(m) for m in self.s.cfg['models']}
        for model in current_models.values():
            model['parameters']['base_url'] = 'http://changed.invalid/v1'
        self.store.model = current_models.get
        original = self.s.request
        def upstream(method, path, **kwargs):
            if path.endswith('/hybrid-search'):
                return {'data': [{'id': 'chunk-one', 'knowledge_id': receipt['knowledge_id'], 'content': 'local fixture'}]}
            return original(method, path, **kwargs)
        self.s.request = upstream
        self.s.client = httpx.Client(transport=httpx.MockTransport(lambda request: self.fail('provider must not be called')))
        self.addCleanup(self.s.client.close)
        execution = self.s.search(Search(knowledge_base_id=KB, source_scope='pipeline', query_text='local fixture'))['execution']
        self.assertEqual(execution['embedding_error_code'], 'model_configuration_changed')
        self.assertTrue(execution['degraded'])
        self.assertNotIn('changed.invalid', json.dumps(execution))
        # search is recall only; reranking of verified texts refuses a drifted model configuration the same way
        with self.assertRaises(HTTPException) as error:
            self.s.rerank(Rerank(knowledge_base_id=KB, source_scope='pipeline', query_text='q', documents=['a']))
        self.assertEqual(error.exception.status_code, 409)
        self.assertNotIn('changed.invalid', str(error.exception.detail))

    def test_artifact_key_is_optional_identity_and_keeps_v1_ids(self):
        from .service import publication_id
        base = payload()
        plain = Publication(**base).model_dump(exclude_none=True)
        self.assertNotIn('artifact_key', plain)
        v1 = publication_id({**plain, 'knowledge_base_id': base['knowledge_base_id']})
        a = publication_id({**plain, 'artifact_key': 'proc.v1:' + 'a' * 64})
        b = publication_id({**plain, 'artifact_key': 'proc.v2:' + 'b' * 64})
        self.assertEqual(len({v1, a, b}), 3)
        # two artifacts of one fixed version are two publications, both accepted
        first = self.s.publish(Publication(**{**base, 'artifact_key': 'proc.v1:' + 'a' * 64}))
        second = self.s.publish(Publication(**{**base, 'content': 'reparsed text', 'content_sha256': None,
                                                'artifact_key': 'proc.v2:' + 'b' * 64}))
        self.assertNotEqual(first['publication_id'], second['publication_id'])
        self.assertEqual(second['identity']['artifact_key'], 'proc.v2:' + 'b' * 64)

