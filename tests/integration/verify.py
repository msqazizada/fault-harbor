"""Check only this test's resources; never print unrelated inventory."""
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    raw = stream.read()
assert "must-not-appear-in-inventory" not in raw, "sensitive evidence leaked"
snapshot = json.loads(raw)
assert snapshot["source"] == {
    "environment": "integration", "provider": "docker", "instance": "fixture-engine"
}
assert snapshot["state"] == "complete", "collection is incomplete"
project = sys.argv[2]
containers = {c["name"]: c for c in snapshot["containers"]}

def service(name):
    return containers[f"{project}-{name}-1"]

try:
    assert service("healthy")["health"]["status"] == "healthy"
    assert service("unhealthy")["health"]["status"] == "unhealthy"
    assert service("stopped")["state"] == "exited"
    assert service("stopped")["exitCode"] == 23
    assert service("restarting")["restartCount"] > 0
    assert service("restarting")["state"] == "restarting"
    network_ids = {n["ref"]["id"] for n in snapshot["networks"]}
    assert service("healthy")["networks"]
    for name in ("healthy", "unhealthy", "stopped", "restarting"):
        item = service(name)
        assert snapshot["startedAt"] <= item["observedAt"] <= snapshot["observedAt"]
        assert item["ref"]["source"] == snapshot["source"]
        for attachment in item["networks"]:
            assert attachment["network"]["id"] in network_ids
except (AssertionError, KeyError):
    sys.exit(1)
