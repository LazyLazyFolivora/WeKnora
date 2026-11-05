"""Bounded, explicit version inputs; the server's source map grants access."""
from typing import Literal
from pydantic import BaseModel, ConfigDict, Field, field_validator

class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")

class Identity(StrictModel):
    source_instance: str = Field(min_length=1, max_length=128)
    source_id: str = Field(min_length=1, max_length=128)
    source_record_id: str = Field(min_length=1, max_length=512)
    version_id: str = Field(min_length=1, max_length=128)
    raw_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    # 同一固定版本的派生正文产物（Fabric 附件：处理器版本:正文摘要）。带上时它是身份的一部分，
    # 处理器升级后的新正文是另一条发布；不带时身份与 weknora-kdd-v1 完全相同。
    artifact_key: str | None = Field(default=None, min_length=1, max_length=200, pattern=r"^[A-Za-z0-9._:-]+$")

class Locator(StrictModel):
    source_instance: str = Field(min_length=1, max_length=128)
    source_id: str = Field(min_length=1, max_length=128)
    source_record_id: str = Field(min_length=1, max_length=512)
    version_id: str = Field(min_length=1, max_length=128)
    uri: str | None = Field(default=None, max_length=1000)

# Fabric retrieval metadata: descriptive pointers returned with hits (time, document and evidence types,
# identifiers). Not identity, not authorization; Fabric verifies every hit against its own records.
METADATA_KEYS = frozenset({
    "source_id", "source_record_id", "version_id", "published_at", "source_updated_at", "first_seen_at",
    "proof_available_at", "proof_kind", "proof_uri", "doc_type", "doc_category", "evidence_types",
    "processor_version", "artifact_sha256", "identifiers"})

class Publication(Identity):
    is_demo: bool = Field(default=False, strict=True)
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    raw_locator: Locator
    title: str = Field(min_length=1, max_length=500)
    content: str = Field(min_length=1, max_length=2000000)
    content_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    metadata: dict[str, str | list[str]] | None = None

    @field_validator("metadata")
    @classmethod
    def bounded_metadata(cls, value):
        if value is None:
            return value
        if set(value) - METADATA_KEYS:
            raise ValueError("unknown metadata key")
        for item in value.values():
            items = item if isinstance(item, list) else [item]
            if len(items) > 50 or any(len(x) > 500 for x in items):
                raise ValueError("metadata value exceeds bounds")
        return value

class Member(Identity):
    knowledge_id: str | None = Field(default=None, max_length=64)

class Features(StrictModel):
    is_demo: bool = Field(default=False, strict=True)
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    members: list[Member] = Field(min_length=1, max_length=100)
    include_chunks: bool = False

class Search(StrictModel):
    is_demo: bool = Field(default=False, strict=True)
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    query_text: str = Field(min_length=1, max_length=8000)
    # candidate recall only; the caller verifies candidates and reranks the verified ones through /rerank
    top_k: int = Field(default=20, ge=1, le=60)

class Rerank(StrictModel):
    """Rerank texts the caller has already verified (Fabric: authorized, visible, fixed version)."""
    is_demo: bool = Field(default=False, strict=True)
    knowledge_base_id: str = Field(min_length=1, max_length=64)
    source_scope: str = Field(min_length=1, max_length=128)
    query_text: str = Field(min_length=1, max_length=8000)
    documents: list[str] = Field(min_length=1, max_length=60)

    @field_validator("documents")
    @classmethod
    def bounded_documents(cls, value):
        if any(not 1 <= len(x) <= 8000 for x in value):
            raise ValueError("document exceeds bounds")
        return value

class Disable(StrictModel):
    reason: str = Field(min_length=1, max_length=500)
    state: Literal["disabled", "withdrawn", "revoked", "superseded"] = "disabled"
