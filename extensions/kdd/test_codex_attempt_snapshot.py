import json
from pathlib import Path
import pytest
from fastapi import HTTPException
from . import test_service as support
from .models import Publication

def test_status_cannot_attach_a_new_attempt_to_an_old_rejection(monkeypatch):
    case=support.ServiceTests()
    case.setUp()
    try:
        case.reject_next=True
        with pytest.raises(HTTPException):
            case.s.publish(Publication(**support.payload()))
        (pid,)=case.s.journal.all()
        before=case.s.status(pid)
        assert before["upload_status"]=="rejected"
        assert before["last_attempt"]=={"attempt":1,"outcome":"rejected"}
        inspect=case.s.inspect_with_upstream
        def after_row_snapshot(row):
            # Another upload starts after status read the old publication row, before it reads attempts.
            case.s.journal.update(pid,upload_status="sending")
            assert case.s.journal.record_attempt(pid,"in_flight",None,"synthetic retry started")==2
            return inspect(row)
        monkeypatch.setattr(case.s,"inspect_with_upstream",after_row_snapshot)
        receipt=case.s.status(pid)
        assert not (receipt["upload_status"]=="rejected" and receipt["last_attempt"]=={"attempt":2,"outcome":"in_flight"}),receipt
    finally:
        case.doCleanups()
