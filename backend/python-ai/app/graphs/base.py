from dataclasses import dataclass


@dataclass
class GraphRegistry:
    name: str = "base-graph-registry"


def build_graph_registry() -> GraphRegistry:
    return GraphRegistry()
