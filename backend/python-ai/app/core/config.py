from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class AppSettings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = Field(default="python-ai", alias="APP_NAME")
    app_env: str = Field(default="dev", alias="APP_ENV")
    log_level: str = Field(default="INFO", alias="LOG_LEVEL")
    http_port: int = Field(default=8000, alias="HTTP_PORT")
    grpc_host: str = Field(default="0.0.0.0", alias="GRPC_HOST")
    grpc_port: int = Field(default=50051, alias="GRPC_PORT")
    openai_base_url: str = Field(default="https://api.openai.com/v1", alias="OPENAI_BASE_URL")
    openai_api_key: str = Field(default="", alias="OPENAI_API_KEY")
    runtime_api_model: str = Field(default="gpt-4.1-mini", alias="RUNTIME_API_MODEL")
    runtime_api_timeout_seconds: int = Field(default=30, alias="RUNTIME_API_TIMEOUT_SECONDS")
    embedding_provider: str = Field(default="openai_compatible", alias="EMBEDDING_PROVIDER")
    embedding_api_model: str = Field(default="text-embedding-3-small", alias="EMBEDDING_API_MODEL")
    embedding_api_timeout_seconds: int = Field(default=15, alias="EMBEDDING_API_TIMEOUT_SECONDS")
    elasticsearch_url: str = Field(default="http://127.0.0.1:9200", alias="ELASTICSEARCH_URL")
    milvus_address: str = Field(default="127.0.0.1:19530", alias="MILVUS_ADDRESS")
    rag_es_index: str = Field(default="oncall_knowledge_chunks", alias="RAG_ES_INDEX")
    rag_milvus_collection: str = Field(default="oncall_knowledge_chunks", alias="RAG_MILVUS_COLLECTION")
    embedding_model_path: str = Field(default="", alias="EMBEDDING_MODEL_PATH")
    embedding_model_name: str = Field(
        default="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
        alias="EMBEDDING_MODEL_NAME",
    )
    embedding_local_only: bool = Field(default=False, alias="EMBEDDING_LOCAL_ONLY")
    embedding_dimension: int = Field(default=1536, alias="EMBEDDING_DIMENSION")
    rag_fusion_window: int = Field(default=60, alias="RAG_FUSION_WINDOW")


class RuntimeInfo(BaseModel):
    service: str
    env: str
    status: str


def get_settings() -> AppSettings:
    return AppSettings()
