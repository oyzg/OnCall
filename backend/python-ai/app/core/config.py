from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class AppSettings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = Field(default="python-ai", alias="APP_NAME")
    app_env: str = Field(default="dev", alias="APP_ENV")
    log_level: str = Field(default="INFO", alias="LOG_LEVEL")
    http_port: int = Field(default=8000, alias="HTTP_PORT")
    grpc_port: int = Field(default=50051, alias="GRPC_PORT")
    openai_base_url: str = Field(default="", alias="OPENAI_BASE_URL")
    openai_api_key: str = Field(default="", alias="OPENAI_API_KEY")
    elasticsearch_url: str = Field(default="http://127.0.0.1:9200", alias="ELASTICSEARCH_URL")
    milvus_address: str = Field(default="127.0.0.1:19530", alias="MILVUS_ADDRESS")


class RuntimeInfo(BaseModel):
    service: str
    env: str
    status: str


def get_settings() -> AppSettings:
    return AppSettings()
