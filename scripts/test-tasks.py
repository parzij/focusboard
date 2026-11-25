"""Run the task lifecycle against a local FocusBoard API with a test database."""
import argparse
import ipaddress
import json
import urllib.error
import urllib.parse
import urllib.request
import uuid


def run(target, origin):
    host = urllib.parse.urlsplit(target).hostname
    if host != "localhost" and not (host and ipaddress.ip_address(host).is_loopback):
        raise ValueError("Use a loopback API backed by an isolated test database.")

    def request(label, method, path, expected, body=None, access=None):
        headers = {"Origin": origin}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if access is not None:
            headers["Authorization"] = f"Bearer {access}"
        call = urllib.request.Request(
            f"{target.rstrip('/')}/api/v1{path}",
            data=None if body is None else json.dumps(body).encode(),
            headers=headers,
            method=method,
        )
        try:
            response = urllib.request.urlopen(call, timeout=15)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            if response.code != expected or response.headers.get("Cache-Control") != "no-store":
                raise RuntimeError(f"{label}: unexpected HTTP {response.code}")
            content = response.read()
        print(f"{label}: HTTP {expected}")
        if expected == 204:
            assert content == b""
            return None
        return json.loads(content)

    def register(label):
        credentials = {
            "email": f"task-smoke-{uuid.uuid4().hex}@example.com",
            "password": f"smoke-password-{uuid.uuid4().hex}",
        }
        account = request(f"{label} registration", "POST", "/auth/register", 201,
                          {"name": label, **credentials})
        return account, credentials

    owner, credentials = register("Task Owner")
    login = request("Owner login", "POST", "/auth/login", 200, credentials)
    access = login["accessToken"]
    other, _ = register("Other Account")
    other_access = other["accessToken"]

    assert request("Empty board", "GET", "/tasks", 200, access=access) == {"tasks": []}
    first = request("Create first task", "POST", "/tasks", 201, {"title": "First"}, access)
    second = request("Append a task", "POST", "/tasks", 201,
                     {"title": "Second", "dueDate": "2000-02-29"}, access)
    assert first["position"] == 0 and second["position"] == 1
    assert request("Read board", "GET", "/tasks", 200, access=access) == {"tasks": [first, second]}
    path = f"/tasks/{second['id']}"
    assert request("Read task", "GET", path, 200, access=access) == second
    updated = request("Edit and clear deadline", "PATCH", path, 200,
                      {"description": "Updated", "dueDate": None}, access)
    assert updated["description"] == "Updated" and updated["dueDate"] is None
    board = request("Move to Done", "PATCH", f"{path}/move", 200,
                    {"status": "done", "position": 0}, access)["tasks"]
    assert [(task["id"], task["status"], task["position"]) for task in board] == [
        (first["id"], "not_started", 0), (second["id"], "done", 0),
    ]

    assert request("Other board", "GET", "/tasks", 200, access=other_access) == {"tasks": []}
    for method, endpoint, payload in [
        ("GET", path, None),
        ("PATCH", path, {"title": "Foreign edit"}),
        ("PATCH", f"{path}/move", {"status": "finish", "position": 0}),
        ("DELETE", path, None),
    ]:
        result = request(f"Reject foreign {method}",
                         method, endpoint, 404, payload, other_access)
        assert result["error"]["code"] == "TASK_NOT_FOUND"
    result = request("Reject supplied owner", "POST", "/tasks", 400,
                     {"title": "Foreign create", "userId": owner["user"]["id"]}, other_access)
    assert result["error"]["code"] == "VALIDATION_ERROR"

    request("Soft delete", "DELETE", path, 204, access=access)
    assert request("Board after delete", "GET", "/tasks", 200, access=access) == {"tasks": [first]}
    result = request("Deleted task unavailable", "GET", path, 404, access=access)
    assert result["error"]["code"] == "TASK_NOT_FOUND"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", help="Loopback API base URL before /api/v1")
    parser.add_argument("origin", help="Value configured in APP_ORIGIN")
    args = parser.parse_args()
    run(args.target, args.origin)
