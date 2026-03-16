from dataclasses import dataclass


@dataclass
class ChainRegistry:
    name: str = "base-chain-registry"


def build_chain_registry() -> ChainRegistry:
    return ChainRegistry()
