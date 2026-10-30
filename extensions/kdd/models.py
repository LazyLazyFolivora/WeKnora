"""Bounded, explicit version inputs; the server's source map grants access."""
from typing import Literal
from pydantic import BaseModel, ConfigDict, Field

class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")

class Identity(StrictModel):
    source_instance: str = Field(min_length=1, max_length=128)
    source_id: str = Field(min_length=1, max_length=128)
    source_record_id: str = Field(min_length=1, max_length=512)
    version_id: str = Field(min_length=1, max_length=128)
    raw_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")

class Locator(StrictModel):
    source_instance: str = Field(min_length=1, max_length=128)
    source_id: str = Field(min_length=1, max_length=128)
    source_record_id: str = Field(min_length=1, max_length=512)
    version_id: str = Field(min_length=1, max_length=128)
    uri: str | None = Field(default=None, max_length=1000)

class Publication(Identity):
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    raw_locator: Locator
    title: str = Field(min_length=1, max_length=500)
    content: str = Field(min_length=1, max_length=2000000)
    content_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")

class Member(Identity):
    knowledge_id: str | None = Field(default=None, max_length=64)

class Features(StrictModel):
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    members: list[Member] = Field(min_length=1, max_length=100)
    include_chunks: bool = False

class Search(StrictModel):
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    query_text: str = Field(min_length=1, max_length=8000)
    top_k: int = Field(default=10, ge=1, le=20)
    rerank: bool = True

class Disable(StrictModel):
    reason: str = Field(min_length=1, max_length=500)
    state: Literal["disabled", "withdrawn", "revoked", "superseded"] = "disabled"
