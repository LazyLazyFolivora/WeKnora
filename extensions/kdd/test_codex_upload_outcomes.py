import json
import httpx
import pytest
from . import test_service as base
from .models import Publication
from .service import UpstreamError,upstream_reason

@pytest.fixture
def h():
    case=base.ServiceTests('runTest');case.setUp()
    try:yield case
    finally:case.doCleanups()

def test_accepted_post_then_verification_404_never_resends(h):
    upstream=h.s.request;failed=False
    def request(method,path,**kw):
        nonlocal failed
        if method=='GET' and path.startswith('/api/v1/knowledge/') and not failed:
            failed=True
            raise UpstreamError('verification unavailable',404,'http_404')
        return upstream(method,path,**kw)
    h.s.request=request
    with pytest.raises(UpstreamError):h.s.publish(Publication(**base.payload()))
    assert h.uploads==1
    h.s.publish(Publication(**base.payload()))
    assert h.uploads==1, 'POST was accepted; a later GET failure caused a second physical upload'

def test_http_408_after_acceptance_stays_unknown_and_reconciles(h):
    upstream=h.s.request;failed=False
    def request(method,path,**kw):
        nonlocal failed
        result=upstream(method,path,**kw)
        if path.endswith('/knowledge/file') and not failed:
            failed=True
            raise UpstreamError('timeout response',408,'http_408')
        return result
    h.s.request=request
    with pytest.raises(UpstreamError):h.s.publish(Publication(**base.payload()))
    h.s.publish(Publication(**base.payload()))
    assert h.uploads==1, 'uncertain HTTP408 was marked rejected and uploaded again'

def test_crash_during_retry_of_rejected_upload_never_resends(h):
    upstream=h.s.request
    def reject(method,path,**kw):
        if path.endswith('/knowledge/file'):
            raise UpstreamError('invalid metadata',400,'code1000 Invalid metadata format')
        return upstream(method,path,**kw)
    h.s.request=reject
    with pytest.raises(UpstreamError):h.s.publish(Publication(**base.payload()))
    def crash(method,path,**kw):
        result=upstream(method,path,**kw)
        if path.endswith('/knowledge/file'):raise SystemExit('simulated process loss after upstream accepted')
        return result
    h.s.request=crash
    with pytest.raises(SystemExit):h.s.publish(Publication(**base.payload()))
    assert h.uploads==1
    h.s.request=upstream
    h.s.publish(Publication(**base.payload()))
    assert h.uploads==1, 'retry left durable status rejected during its new physical POST'

def test_error_category_does_not_store_upstream_details():
    response=httpx.Response(400,json={'error':{'code':1000,'message':'Invalid metadata format','details':'PRIVATE_SOURCE_BODY_MARKER'}})
    category=upstream_reason(response)
    assert 'PRIVATE_SOURCE_BODY_MARKER' not in category, 'raw upstream details copied into supposedly safe journal category'
