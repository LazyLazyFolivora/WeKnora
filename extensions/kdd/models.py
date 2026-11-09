"""Bounded, explicit version inputs; the server's source map grants access."""
import logging
from typing import Literal
from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

log = logging.getLogger(__name__)

class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")

class Piece(StrictModel):
    """One index piece of a long derived artifact (Fabric processing.index_pieces): the artifact keeps its
    artifact_key; the piece names its rule version, position, absolute range in the full text, the length of a
    context prefix (a repeated table header, not source text), its size and the sha256 of the uploaded content."""
    rule: str = Field(min_length=1, max_length=100, pattern=r"^[A-Za-z0-9._:-]+$")
    index: int = Field(ge=1)
    count: int = Field(ge=1)
    start: int = Field(ge=0)
    end: int = Field(ge=1)
    context_chars: int = Field(ge=0)
    chars: int = Field(ge=1)
    bytes: int = Field(ge=1)
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")

    @model_validator(mode="after")
    def consistent(self):
        if not (self.index <= self.count and self.start < self.end and self.chars == self.end - self.start
                + self.context_chars):
            raise ValueError("inconsistent piece")
        return self

class Identity(StrictModel):
    source_instance: str = Field(min_length=1, max_length=128)
    source_id: str = Field(min_length=1, max_length=128)
    source_record_id: str = Field(min_length=1, max_length=512)
    version_id: str = Field(min_length=1, max_length=128)
    raw_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    # 同一固定版本的派生正文产物（Fabric 附件：处理器版本:正文摘要）。带上时它是身份的一部分，
    # 处理器升级后的新正文是另一条发布；不带时身份与 weknora-kdd-v1 完全相同。
    artifact_key: str | None = Field(default=None, min_length=1, max_length=200, pattern=r"^[A-Za-z0-9._:-]+$")
    # a piece of that artifact (long artifacts only); part of the identity when present, absent otherwise
    piece: Piece | None = None

class Locator(StrictModel):
    source_instance: str = Field(min_length=1, max_length=128)
    source_id: str = Field(min_length=1, max_length=128)
    source_record_id: str = Field(min_length=1, max_length=512)
    version_id: str = Field(min_length=1, max_length=128)
    uri: str | None = Field(default=None, max_length=1000)

# Fabric retrieval metadata: descriptive pointers returned with hits (time, document and evidence types,
# identifiers). Not identity, not authorization; Fabric verifies every hit against its own records.
# These are the keys we know today. A key not listed here is logged and kept, never a reason to refuse.
METADATA_KEYS = frozenset({
    "source_id", "source_record_id", "version_id", "published_at", "published_at_meaning", "source_updated_at",
    "source_updated_at_meaning", "first_seen_at", "proof_available_at", "proof_kind", "proof_uri", "doc_type",
    "doc_category", "evidence_types", "processor_version", "artifact_sha256", "identifiers"})

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
        unknown = set(value) - METADATA_KEYS
        if unknown:
            log.warning("unknown metadata keys accepted as descriptive: %s", [k[:60] for k in sorted(unknown)[:10]])
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
    # one recall over the knowledge base for every requested scope: scopes sharing a KB do not crowd each other out
    source_scopes: list[str] = Field(min_length=1, max_length=8)
    query_text: str = Field(min_length=1, max_length=8000)

    @field_validator("source_scopes")
    @classmethod
    def bounded_scopes(cls, value):
        if any(not 1 <= len(x) <= 128 for x in value) or len(set(value)) != len(value):
            raise ValueError("invalid source scopes")
        return value
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
