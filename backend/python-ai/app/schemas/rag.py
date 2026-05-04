from __future__ import annotations

from pydantic import BaseModel, Field


class RAGChunkInput(BaseModel):
    document_id: str
    document_title: str
    category: str
    index: int
    content: str


class RAGIndexRequest(BaseModel):
    user_id: str
    document_id: str
    document_title: str
    category: str
    chunks: list[RAGChunkInput] = Field(default_factory=list)


class RAGIndexResponse(BaseModel):
    status: str
    indexed_chunks: int
    embedding_backend: str
    vector_backend: str
    lexical_backend: str


class RAGDeleteRequest(BaseModel):
    document_id: str


class RAGDeleteResponse(BaseModel):
    status: str
    document_id: str


class RAGRetrieveRequest(BaseModel):
    query: str
    category: str = ""
    limit: int = 4


class RAGReference(BaseModel):
    document_id: str
    document_title: str
    category: str
    chunk_index: int
    chunk: str
    score: float
    lexical_score: float = 0.0
    semantic_score: float = 0.0
    boost_score: float = 0.0
    match_reasons: list[str] = Field(default_factory=list)


class RAGRetrieveResponse(BaseModel):
    query: str
    rewritten_query: str
    query_terms: list[str] = Field(default_factory=list)
    expanded_terms: list[str] = Field(default_factory=list)
    answer: str
    references: list[RAGReference] = Field(default_factory=list)
    scanned_docs: int = 0
    scanned_chunks: int = 0
    matched_chunks: int = 0
    lexical_candidates: int = 0
    semantic_candidates: int = 0
    reranked_chunks: int = 0
    strategy: str = "embedding_milvus_es_hybrid"
    requested_limit: int = 4
    embedding_backend: str = ""
    vector_backend: str = ""
    lexical_backend: str = ""
