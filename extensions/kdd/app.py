"""Run with one uvicorn worker; private token and source map are required."""
import hmac
import os
from fastapi import Depends, FastAPI, Header, HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from .models import Publication, Features, Search, Disable, Rerank
from .service import Service, CONTRACT

def create_app(config_path):
    service = Service(config_path)
    def auth(authorization: str | None = Header(default=None)):
        expected = "Bearer " + service.config()["token"]
        if not authorization or not hmac.compare_digest(authorization, expected):
            raise HTTPException(401, "valid service token required")
    app = FastAPI(title="WeKnora fixed-version KDD extension", version=CONTRACT,
                  dependencies=[Depends(auth)])
    app.state.service = service
    @app.exception_handler(RequestValidationError)
    async def validation_error(request: Request, exc):
        # Do not echo submitted document content or credentials in errors.
        return JSONResponse(status_code=400, content={"detail": "invalid request fields or bounds", "fields": [list(e["loc"]) for e in exc.errors()]})
    @app.post("/api/v1/kdd/publications")
    def publish(req: Publication):
        return service.publish(req)
    @app.get("/api/v1/kdd/publications/{pid}")
    def status(pid: str):
        return service.status(pid)
    @app.post("/api/v1/kdd/publications/{pid}/disable")
    def disable(pid: str, req: Disable):
        return service.disable(pid, req)
    @app.post("/api/v1/kdd/features")
    def features(req: Features):
        return service.features(req)
    @app.post("/api/v1/kdd/search")
    def search(req: Search):
        return service.search(req)
    @app.post("/api/v1/kdd/rerank")
    def rerank(req: Rerank):
        return service.rerank(req)
    @app.get("/health")
    def health():
        return {"status": "ok", "contract_version": CONTRACT}
    return app

def app_factory():
    return create_app(os.environ["WEKNORA_KDD_CONFIG"])
