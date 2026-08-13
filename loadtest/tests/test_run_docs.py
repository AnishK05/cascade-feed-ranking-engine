from pathlib import Path

REPO = Path(__file__).resolve().parents[1].parent
DOCS = REPO / "docs"


def test_run_docs_exist_and_point_at_the_demo():
    required = (
        "README.md",
        "running.md",
        "running-on-windows.md",
        "demo.md",
        "ports-and-configuration.md",
        "architecture.md",
    )
    for name in required:
        path = DOCS / name
        assert path.is_file(), f"missing {path}"
    running = (DOCS / "running.md").read_text(encoding="utf-8")
    assert "cascade.cmd up" in running
    assert "make up" in running
    assert "make smoke" in running
    readme = (REPO / "README.md").read_text(encoding="utf-8")
    assert "docs/running.md" in readme
    assert "docs/demo.md" in readme
