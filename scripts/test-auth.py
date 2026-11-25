"""Run the authentication lifecycle against a running local FocusBoard API."""
import argparse
import http.cookiejar
import json
import urllib.error
import urllib.request
import uuid


def run(target, origin):
    cookies = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookies))
    email = f"smoke-{uuid.uuid4().hex}@example.com"
    password = f"smoke-password-{uuid.uuid4().hex}"
    new_password = f"updated-password-{uuid.uuid4().hex}"

    def request(label, method, path, expected, body=None, access=None, replay=None):
        headers = {"Origin": origin}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if access is not None:
            headers["Authorization"] = f"Bearer {access}"
        if replay is not None:
            headers["Cookie"] = replay
        call = urllib.request.Request(
            f"{target.rstrip('/')}/api/v1{path}",
            data=None if body is None else json.dumps(body).encode(),
            headers=headers,
            method=method,
        )
        try:
            response = client.open(call, timeout=15)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            if response.code != expected or response.headers.get("Cache-Control") != "no-store":
                raise RuntimeError(f"{label}: unexpected HTTP {response.code}")
            result = json.loads(response.read()) if response.code != 204 else None
        print(f"{label}: HTTP {expected}")
        return result

    def current_cookie():
        (cookie,) = cookies
        if cookie.path != "/api/v1/auth" or cookie.domain_specified or not cookie.has_nonstandard_attr("HttpOnly"):
            raise RuntimeError("Unexpected refresh cookie scope")
        return f"{cookie.name}={cookie.value}"

    registration = request("Registration", "POST", "/auth/register", 201,
                           {"name": "Smoke Example", "email": email, "password": password})
    assert registration["user"]["email"] == email
    current_cookie()
    login = request("Login", "POST", "/auth/login", 200, {"email": email, "password": password})
    profile = request("Profile", "GET", "/users/me", 200, access=login["accessToken"])
    assert profile["id"] == registration["user"]["id"]

    old_cookie = current_cookie()
    refreshed = request("Refresh", "POST", "/auth/refresh", 200)
    assert current_cookie() != old_cookie
    request("Old refresh rejected", "POST", "/auth/refresh", 401, replay=old_cookie)
    updated = request("Profile update", "PATCH", "/users/me", 200,
                      {"name": "Smoke Updated"}, access=refreshed["accessToken"])
    assert updated["name"] == "Smoke Updated" and updated["id"] == profile["id"]
    password_cookie = current_cookie()
    request("Password change", "PUT", "/users/me/password", 204,
            {"currentPassword": password, "newPassword": new_password}, access=refreshed["accessToken"])
    assert not list(cookies)
    request("Revoked refresh rejected", "POST", "/auth/refresh", 401, replay=password_cookie)
    request("Old password rejected", "POST", "/auth/login", 401, {"email": email, "password": password})
    request("Login with new password", "POST", "/auth/login", 200, {"email": email, "password": new_password})
    logout_cookie = current_cookie()
    request("Logout", "POST", "/auth/logout", 204)
    assert not list(cookies)
    request("Logged-out refresh rejected", "POST", "/auth/refresh", 401, replay=logout_cookie)
    request("Repeated logout", "POST", "/auth/logout", 204)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", help="API base URL before /api/v1")
    parser.add_argument("origin", help="Value configured in APP_ORIGIN")
    args = parser.parse_args()
    run(args.target, args.origin)
