from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
K8S = ROOT.parent / "deploy" / "k8s"

CLOUD_NEEDLES = (
    "eks.amazonaws.com",
    "gke.io",
    "cloud.google.com",
    "kubernetes.azure.com",
    "elasticloadbalancing",
)


def test_k8s_manifests_are_local_kind_only():
    files = list(K8S.glob("*.yaml"))
    names = {path.name for path in files}
    for required in (
        "kind-config.yaml",
        "kustomization.yaml",
        "feed-hpa.yaml",
        "metrics-server.yaml",
    ):
        assert required in names, f"missing {required}"
    text = "\n".join(path.read_text() for path in files)
    for needle in CLOUD_NEEDLES:
        assert needle not in text, f"cloud provider residue: {needle}"
    assert "kind: HorizontalPodAutoscaler" in text
    assert "secretGenerator:" in text
    assert "hostPort: 8080" in text
    assert "imagePullPolicy: Never" in text
    assert "cascade/feed-service:local" in text


def test_k8s_init_files_match_canonical_sources():
    """Kustomize cannot load files outside deploy/k8s; copies must not drift."""
    pairs = (
        ("init/001-users.sql", "migrations/000001_create_users_table.up.sql"),
        ("init/002-follows.sql", "migrations/000002_create_follows_table.up.sql"),
        ("init/003-posts.sql", "migrations/000003_create_posts_table.up.sql"),
        ("init/004-engagements.sql", "migrations/000004_create_engagements_table.up.sql"),
        ("init/kafka-init.sh", "scripts/kafka-init.sh"),
    )
    repo = ROOT.parent
    for relative, canonical in pairs:
        got = (K8S / relative).read_bytes()
        want = (repo / canonical).read_bytes()
        assert got == want, f"{relative} drifted from {canonical}"
