package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parzij/focusboard/backend/internal/app"
	"github.com/parzij/focusboard/backend/internal/auth"
	"github.com/parzij/focusboard/backend/internal/config"
	"github.com/parzij/focusboard/backend/internal/platform/token"
	"github.com/parzij/focusboard/backend/internal/users"
)

const (
	origin          = "https://focusboard.example.com"
	initialPassword = "  original passphrase  "
	updatedPassword = "updated passphrase"
	cookieName      = "focusboard_refresh"
)

type authAPI struct {
	router http.Handler
}

type authResponse struct {
	AccessToken string     `json:"accessToken"`
	ExpiresIn   int        `json:"expiresIn"`
	User        users.User `json:"user"`
	cookie      *http.Cookie
}

func TestAuthenticationPostgreSQL(t *testing.T) {
	pool, provider := testDatabase(t, 10)
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	router, err := app.Router(config.Config{
		Database: config.Database{CheckTimeout: time.Second},
		Auth: config.Auth{
			Secret: []byte("integration-signing-fixture-only"),
			Issuer: "integration", Audience: "integration-web", Origin: origin,
			CookieName: cookieName, CookieSecure: true,
			AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour, RequestTimeout: 20 * time.Second,
			IPLimit: 200, AccountLimit: 20, RateWindow: time.Minute, RateKeys: 100, PasswordWorkers: 4,
		},
	}, pool)
	if err != nil {
		t.Fatal(err)
	}
	api := authAPI{router: router}

	t.Run("registration login and profile isolation", func(t *testing.T) {
		alex := api.register(t, "  ALEX@example.com  ")
		if alex.User.Email != "alex@example.com" || alex.User.Name != "Example Name" || alex.ExpiresIn != 900 ||
			alex.User.CreatedAt.IsZero() || time.Until(alex.cookie.Expires) < 29*24*time.Hour {
			t.Fatal("registration did not normalize and initialize the session")
		}
		hash, _ := token.RefreshHash(alex.cookie.Value)
		var storedHash string
		if err := pool.QueryRow(t.Context(), `SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, alex.User.ID).Scan(&storedHash); err != nil || storedHash != hash {
			t.Fatalf("registered session was not stored as a digest: %v", err)
		}

		bob := api.register(t, "bob@example.com")
		checkAuthResponse(t, api.call(t, "POST", "/auth/register", registerBody("alex@example.com"), origin, "", nil), 409, "EMAIL_ALREADY_EXISTS")
		checkAuthResponse(t, api.call(t, "POST", "/auth/register", `{"name":" ","email":"invalid","password":"short"}`, origin, "", nil), 422, "VALIDATION_ERROR")
		wrong := api.call(t, "POST", "/auth/login", loginBody("alex@example.com", strings.TrimSpace(initialPassword)), origin, "", nil)
		missing := api.call(t, "POST", "/auth/login", loginBody("absent@example.com", initialPassword), origin, "", nil)
		checkAuthResponse(t, wrong, 401, "INVALID_CREDENTIALS")
		checkAuthResponse(t, missing, 401, "INVALID_CREDENTIALS")
		if wrong.Body.String() != missing.Body.String() || !strings.Contains(wrong.Body.String(), "Invalid email or password.") {
			t.Fatal("credential failures exposed account existence")
		}
		login := readAuth(t, api.call(t, "POST", "/auth/login", loginBody(" AlEX@example.com ", initialPassword), origin, "", nil), 200)
		if login.User.ID != alex.User.ID || login.cookie.Value == alex.cookie.Value {
			t.Fatal("login session mismatch")
		}
		checkAuthResponse(t, api.call(t, "GET", "/users/me?userId="+bob.User.ID, "", "", "", nil), 401, "UNAUTHORIZED")
		profile := api.call(t, "GET", "/users/me?userId="+bob.User.ID, "", "", alex.AccessToken, nil)
		checkAuthResponse(t, profile, 200, "alex@example.com")
		checkAuthResponse(t, api.call(t, "PATCH", "/users/me?userId="+bob.User.ID, `{"name":" Alex Updated "}`, "", alex.AccessToken, nil), 200, "Alex Updated")
		checkAuthResponse(t, api.call(t, "GET", "/users/me", "", "", bob.AccessToken, nil), 200, "Example Name")
		checkAuthResponse(t, api.call(t, "PATCH", "/users/me", `{"name":" "}`, "", alex.AccessToken, nil), 422, "VALIDATION_ERROR")
		checkAuthResponse(t, api.call(t, "PATCH", "/users/me", `{"name":"Fine","userId":"`+bob.User.ID+`"}`, "", alex.AccessToken, nil), 400, "VALIDATION_ERROR")
	})
	t.Run("origin and JSON boundaries", func(t *testing.T) {
		for _, from := range []string{"", "null", "https://wrong.example.com", origin + "/"} {
			result := api.call(t, "POST", "/auth/register", registerBody("origin@example.com"), from, "", nil)
			checkAuthResponse(t, result, 403, "FORBIDDEN")
		}
		for _, body := range []string{
			"null", "[]", "{", "{} {}", `{"email":"bad","extra":true}`, `{"password":"a","password":"b"}`,
			`{"password":"\ud800"}`, `{"password":"` + strings.Repeat("x", 64<<10) + `"}`,
		} {
			checkAuthResponse(t, api.call(t, "POST", "/auth/login", body, origin, "", nil), 400, "VALIDATION_ERROR")
		}
		for _, path := range []string{"/auth/logout", "/auth/refresh"} {
			checkAuthResponse(t, api.call(t, "POST", path, `{"refreshToken":"example"}`, origin, "", nil), 400, "VALIDATION_ERROR")
			checkAuthResponse(t, api.call(t, "POST", path, "", "", "", nil), 403, "FORBIDDEN")
		}
	})
	t.Run("account rate limit uses normalized email", func(t *testing.T) {
		for range 20 {
			result := api.call(t, "POST", "/auth/login", loginBody("Limit@example.com", ""), origin, "", nil)
			checkAuthResponse(t, result, 401, "INVALID_CREDENTIALS")
		}
		result := api.call(t, "POST", "/auth/login", loginBody("  LIMIT@example.com  ", ""), origin, "", nil)
		checkAuthResponse(t, result, 429, "RATE_LIMITED")
		if result.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	})

	t.Run("concurrent registration has one winner", func(t *testing.T) {
		results := concurrentRequests(t,
			func() *httptest.ResponseRecorder {
				return api.call(t, "POST", "/auth/register", registerBody("race@example.com"), origin, "", nil)
			},
			func() *httptest.ResponseRecorder {
				return api.call(t, "POST", "/auth/register", registerBody("RACE@example.com"), origin, "", nil)
			},
		)
		assertStatuses(t, results, 201, 409)
		assertCount(t, pool, 1, `SELECT count(*) FROM refresh_tokens s JOIN users u ON u.id=s.user_id WHERE u.email=$1`, "race@example.com")
	})
	t.Run("refresh absolute expiry concurrency and logout", func(t *testing.T) {
		session := api.register(t, "refresh@example.com")
		checkAuthResponse(t, api.call(t, "POST", "/auth/refresh", "", origin, "", nil), 401, "UNAUTHORIZED")
		results := concurrentRequests(t,
			func() *httptest.ResponseRecorder {
				return api.call(t, "POST", "/auth/refresh", "", origin, "", session.cookie)
			},
			func() *httptest.ResponseRecorder {
				return api.call(t, "POST", "/auth/refresh", "", origin, "", session.cookie)
			},
		)
		assertStatuses(t, results, 200, 401)
		if results[0].Code != 200 {
			results[0], results[1] = results[1], results[0]
		}
		result := readAuth(t, results[0], 200)
		if !result.cookie.Expires.Equal(session.cookie.Expires) || result.cookie.Value == session.cookie.Value || results[1].Header().Get("Set-Cookie") != "" {
			t.Fatal("rotation changed expiry, reused the cookie, or the loser overwrote it")
		}
		oldHash, _ := token.RefreshHash(session.cookie.Value)
		newHash, _ := token.RefreshHash(result.cookie.Value)
		assertCount(t, pool, 1, `SELECT count(*) FROM refresh_tokens old JOIN refresh_tokens next ON next.user_id=old.user_id
			WHERE old.token_hash=$1 AND next.token_hash=$2 AND old.expires_at=next.expires_at AND old.revoked_at IS NOT NULL AND next.revoked_at IS NULL`, oldHash, newHash)
		checkAuthResponse(t, api.call(t, "POST", "/auth/refresh", "", origin, "", session.cookie), 401, "UNAUTHORIZED")
		for range 2 {
			logout := api.call(t, "POST", "/auth/logout", "", origin, "", result.cookie)
			checkAuthResponse(t, logout, 204, "")
			checkCookie(t, logout, true)
		}
		checkAuthResponse(t, api.call(t, "POST", "/auth/refresh", "", origin, "", result.cookie), 401, "UNAUTHORIZED")
		expired := api.register(t, "expired@example.com")
		expiredHash, _ := token.RefreshHash(expired.cookie.Value)
		execSQL(t, pool, "UPDATE refresh_tokens SET expires_at = now() - interval '1 second' WHERE token_hash = $1", expiredHash)
		checkAuthResponse(t, api.call(t, "POST", "/auth/refresh", "", origin, "", expired.cookie), 401, "UNAUTHORIZED")
	})
	t.Run("password change revokes sessions and serializes concurrent writers", func(t *testing.T) {
		session := api.register(t, "password@example.com")
		secondSession := readAuth(t, api.call(t, "POST", "/auth/login", loginBody(session.User.Email, initialPassword), origin, "", nil), 200)
		other := api.register(t, "unaffected@example.com")
		_, oldHash, err := auth.NewRepository(pool).CredentialsByEmail(t.Context(), session.User.Email)
		if err != nil {
			t.Fatal(err)
		}

		change := passwordBody(initialPassword, updatedPassword)
		checkAuthResponse(t, api.call(t, "PUT", "/users/me/password", change, "", session.AccessToken, nil), 403, "FORBIDDEN")
		checkAuthResponse(t, api.call(t, "PUT", "/users/me/password", passwordBody("wrong", updatedPassword), origin, session.AccessToken, nil), 401, "INVALID_CREDENTIALS")
		checkAuthResponse(t, api.call(t, "PUT", "/users/me/password", passwordBody(initialPassword, "short"), origin, session.AccessToken, nil), 422, "VALIDATION_ERROR")
		results := concurrentRequests(t,
			func() *httptest.ResponseRecorder {
				return api.call(t, "PUT", "/users/me/password", change, origin, session.AccessToken, nil)
			},
			func() *httptest.ResponseRecorder {
				return api.call(t, "POST", "/auth/login", loginBody(session.User.Email, initialPassword), origin, "", nil)
			},
			func() *httptest.ResponseRecorder {
				return api.call(t, "POST", "/auth/refresh", "", origin, "", session.cookie)
			},
		)
		checkAuthResponse(t, results[0], 204, "")
		checkCookie(t, results[0], true)
		for _, result := range results[1:] {
			if result.Code == 200 {
				issued := readAuth(t, result, 200)
				checkAuthResponse(t, api.call(t, "POST", "/auth/refresh", "", origin, "", issued.cookie), 401, "UNAUTHORIZED")
			} else {
				checkAuthResponse(t, result, 401, "")
			}
		}
		for _, old := range []*http.Cookie{session.cookie, secondSession.cookie} {
			checkAuthResponse(t, api.call(t, "POST", "/auth/refresh", "", origin, "", old), 401, "UNAUTHORIZED")
		}
		assertCount(t, pool, 0, "SELECT count(*) FROM refresh_tokens WHERE user_id=$1 AND revoked_at IS NULL", session.User.ID)
		_, staleDigest, err := token.NewRefresh()
		if err != nil {
			t.Fatal(err)
		}
		_, err = auth.NewRepository(pool).Login(t.Context(), session.User.ID, oldHash, staleDigest, time.Now().Add(time.Hour))
		if err != auth.ErrCredentials {
			t.Fatalf("stale login credential snapshot accepted: %v", err)
		}
		checkAuthResponse(t, api.call(t, "POST", "/auth/login", loginBody(session.User.Email, initialPassword), origin, "", nil), 401, "INVALID_CREDENTIALS")
		readAuth(t, api.call(t, "POST", "/auth/login", loginBody(session.User.Email, updatedPassword), origin, "", nil), 200)
		readAuth(t, api.call(t, "POST", "/auth/refresh", "", origin, "", other.cookie), 200)
		checkAuthResponse(t, api.call(t, "GET", "/users/me", "", "", session.AccessToken, nil), 200, session.User.Email)
	})
	t.Run("database errors roll back registration rotation and password change", func(t *testing.T) {
		session := api.register(t, "rollback@example.com")
		execSQL(t, pool, `CREATE FUNCTION reject_auth_test_write() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'fixture_should_never_reach_http'; END
			$$`)
		execSQL(t, pool, `CREATE TRIGGER auth_failure BEFORE INSERT ON refresh_tokens
			FOR EACH ROW EXECUTE FUNCTION reject_auth_test_write()`)
		checkAuthResponse(t, api.call(t, "POST", "/auth/register", registerBody("atomic@example.com"), origin, "", nil), 500, "INTERNAL_ERROR")
		assertCount(t, pool, 0, "SELECT count(*) FROM users WHERE email=$1", "atomic@example.com")
		checkAuthResponse(t, api.call(t, "POST", "/auth/login", loginBody(session.User.Email, initialPassword), origin, "", nil), 500, "INTERNAL_ERROR")
		rejected := api.call(t, "POST", "/auth/refresh", "", origin, "", session.cookie)
		checkAuthResponse(t, rejected, 500, "INTERNAL_ERROR")
		if rejected.Header().Get("Set-Cookie") != "" {
			t.Fatal("failed rotation attempted to replace cookie")
		}
		oldHash, _ := token.RefreshHash(session.cookie.Value)
		assertCount(t, pool, 1, "SELECT count(*) FROM refresh_tokens WHERE token_hash=$1 AND revoked_at IS NULL", oldHash)
		execSQL(t, pool, "DROP TRIGGER auth_failure ON refresh_tokens")
		rotated := readAuth(t, api.call(t, "POST", "/auth/refresh", "", origin, "", session.cookie), 200)

		execSQL(t, pool, `CREATE TRIGGER auth_failure BEFORE UPDATE ON refresh_tokens
			FOR EACH ROW EXECUTE FUNCTION reject_auth_test_write()`)
		logout := api.call(t, "POST", "/auth/logout", "", origin, "", rotated.cookie)
		checkAuthResponse(t, logout, 500, "INTERNAL_ERROR")
		if logout.Header().Get("Set-Cookie") != "" {
			t.Fatal("failed logout hid failure by clearing cookie")
		}
		checkAuthResponse(t, api.call(t, "PUT", "/users/me/password", passwordBody(initialPassword, updatedPassword), origin, session.AccessToken, nil), 500, "INTERNAL_ERROR")
		execSQL(t, pool, "DROP TRIGGER auth_failure ON refresh_tokens")
		readAuth(t, api.call(t, "POST", "/auth/login", loginBody(session.User.Email, initialPassword), origin, "", nil), 200)
	})
}

func (a authAPI) call(t *testing.T, method, path, body, from, access string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "/api/v1"+path, strings.NewReader(body))
	if from != "" {
		request.Header.Set("Origin", from)
	}
	if access != "" {
		request.Header.Set("Authorization", "Bearer "+access)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	a.router.ServeHTTP(recorder, request)
	return recorder
}

func (a authAPI) register(t *testing.T, email string) authResponse {
	t.Helper()
	return readAuth(t, a.call(t, "POST", "/auth/register", registerBody(email), origin, "", nil), 201)
}

func checkAuthResponse(t *testing.T, response *httptest.ResponseRecorder, status int, contains string) {
	t.Helper()
	if response.Code != status || !strings.Contains(response.Body.String(), contains) ||
		response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected status %d and response containing %q, got %d: %s", status, contains, response.Code, response.Body)
	}
	for _, secret := range []string{"fixture_should_never_reach_http", "password_hash", "token_hash", "refreshToken", initialPassword, updatedPassword} {
		if strings.Contains(response.Body.String(), secret) {
			t.Fatal("response exposed credential data")
		}
	}
}

func readAuth(t *testing.T, response *httptest.ResponseRecorder, status int) authResponse {
	t.Helper()
	checkAuthResponse(t, response, status, "")
	var result authResponse
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil || decoder.Decode(new(any)) != io.EOF ||
		result.AccessToken == "" || result.User.ID == "" {
		t.Fatalf("invalid authentication response: %v", err)
	}
	result.cookie = checkCookie(t, response, false)
	return result
}

func checkCookie(t *testing.T, response *httptest.ResponseRecorder, deleted bool) *http.Cookie {
	t.Helper()
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected a single refresh cookie")
	}
	cookie := cookies[0]
	if cookie.Name != cookieName || cookie.Path != "/api/v1/auth" || cookie.Domain != "" ||
		!cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("invalid cookie attributes")
	}
	if deleted && (cookie.MaxAge >= 0 || cookie.Value != "" || !cookie.Expires.Before(time.Now())) ||
		!deleted && (cookie.MaxAge <= 0 || cookie.Value == "" || !cookie.Expires.After(time.Now())) {
		t.Fatal("invalid cookie lifetime")
	}
	return cookie
}

func encodeBody(fields map[string]string) string {
	body, _ := json.Marshal(fields)
	return string(body)
}

func registerBody(email string) string {
	return encodeBody(map[string]string{"name": "  Example Name  ", "email": email, "password": initialPassword})
}

func loginBody(email, secret string) string {
	return encodeBody(map[string]string{"email": email, "password": secret})
}

func passwordBody(current, updated string) string {
	return encodeBody(map[string]string{"currentPassword": current, "newPassword": updated})
}

func concurrentRequests(t *testing.T, calls ...func() *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	t.Helper()
	var group sync.WaitGroup
	start := make(chan struct{})
	results := make([]*httptest.ResponseRecorder, len(calls))
	for i, call := range calls {
		group.Go(func() { <-start; results[i] = call() })
	}
	close(start)
	group.Wait()
	return results
}

func assertStatuses(t *testing.T, responses []*httptest.ResponseRecorder, first, second int) {
	t.Helper()
	a, b := responses[0].Code, responses[1].Code
	if !(a == first && b == second || a == second && b == first) {
		t.Fatalf("concurrent responses: %d, %d; expected %d and %d", a, b, first, second)
	}
}

func execSQL(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}
