from pathlib import Path

REPO = Path(__file__).resolve().parents[1].parent


def test_windows_entrypoints_cover_the_demo_path():
    cmd = (REPO / "cascade.cmd").read_text(encoding="utf-8")
    ps1 = (REPO / "cascade.ps1").read_text(encoding="utf-8")
    assert "cascade.ps1" in cmd
    assert "ExecutionPolicy Bypass" in cmd
    for command in (
        '"up"',
        '"smoke"',
        '"seed"',
        '"kind-up"',
        '"kind-smoke"',
        "Ensure-UnixFile",
        "Wait-GatewayPing",
    ):
        assert command in ps1, f"cascade.ps1 missing {command}"
