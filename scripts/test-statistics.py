"""Run the statistics lifecycle against the local API and an isolated database."""
import argparse
import urllib.parse
import uuid
from datetime import datetime, timezone

from smoke_http import local_requests


def run(target, origin):
    request = local_requests(target, origin)

    def register(label):
        return request(f"{label} registration", "POST", "/auth/register", 201, {
            "name": label,
            "email": f"statistics-smoke-{uuid.uuid4().hex}@example.com",
            "password": f"smoke-password-{uuid.uuid4().hex}",
        })["accessToken"]

    owner = register("Statistics Owner")
    other = register("Statistics Other")

    # Choose a local noon so this scenario can run across UTC day, week or month boundaries.
    offset = 12 - datetime.now(timezone.utc).hour
    zone = "UTC" if offset == 0 else f"Etc/GMT{-offset:+d}"

    def report(label, access, expected, period="days", timezone_name=zone):
        query = urllib.parse.urlencode({"period": period, "timezone": timezone_name})
        data = request(label, "GET", f"/statistics?{query}", 200, access=access)
        assert set(data) == {"period", "timezone", "buckets", "thisWeek", "thisMonth"}
        assert data["period"] == period and data["timezone"] == timezone_name
        assert len(data["buckets"]) == (7 if period == "days" else 8)
        dates = [bucket["startDate"] for bucket in data["buckets"]]
        assert dates == sorted(set(dates))
        assert sum(bucket["count"] for bucket in data["buckets"]) == expected
        assert data["thisWeek"] == expected and data["thisMonth"] == expected
        return data

    empty = report("Empty statistics", owner, 0)
    first = request("Create first task", "POST", "/tasks", 201, {"title": "First"}, owner)
    request("Create second task", "POST", "/tasks", 201, {"title": "Second"}, owner)
    path = f"/tasks/{first['id']}"

    def status(label, value):
        request(label, "PATCH", path, 200, {"status": value}, owner)

    status("Finish task", "finish")
    assert report("Finish excluded", owner, 0) == empty
    status("Close task", "done")
    completed = report("First Done counted", owner, 1)
    status("Reopen task", "in_progress")
    assert report("Reopened task keeps completion", owner, 1) == completed
    status("Close again", "done")
    assert report("Repeated Done excluded", owner, 1) == completed
    request("Delete closed task", "DELETE", path, 204, access=owner)
    assert report("Completion survives delete", owner, 1) == completed
    report("Weekly statistics", owner, 1, "weeks")
    report("Other zone and owner", other, 0, timezone_name="Europe/Moscow")
    assert report("Other owner in the same zone", other, 0) == empty

    error = request("Reject missing bearer", "GET", "/statistics", 401)
    assert error["error"]["code"] == "UNAUTHORIZED"
    for query in ("period=month", "timezone=Local", "period=days&period=days", "userId=example"):
        error = request(f"Reject {query}", "GET", f"/statistics?{query}", 422, access=owner)
        assert error["error"]["code"] == "VALIDATION_ERROR"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", help="Loopback API base URL before /api/v1")
    parser.add_argument("origin", help="Value configured in APP_ORIGIN")
    args = parser.parse_args()
    run(args.target, args.origin)
