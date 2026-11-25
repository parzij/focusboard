"""HTTP helpers for local API smoke scenarios."""
import ipaddress
import json
import urllib.error
import urllib.parse
import urllib.request


def local_requests(target, origin):
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

    return request
