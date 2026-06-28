package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthCookieDomainUsesHaotushowParentDomain(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://workbench.haotushow.com/api/auth/me", nil)
	if got := authCookieDomain(request); got != ".haotushow.com" {
		t.Fatalf("authCookieDomain = %q, want .haotushow.com", got)
	}
}

func TestAuthCookieDomainOmitsLocalhostDomain(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://localhost:3000/api/auth/me", nil)
	if got := authCookieDomain(request); got != "" {
		t.Fatalf("authCookieDomain = %q, want empty domain", got)
	}
}

func TestAuthTokenFromRequestReadsBearerBeforeCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://workbench.haotushow.com/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "cookie-token"})
	request.Header.Set("Authorization", "Bearer header-token")
	if got := AuthTokenFromRequest(request); got != "header-token" {
		t.Fatalf("AuthTokenFromRequest = %q, want header-token", got)
	}
}

func TestAuthTokenFromRequestReadsCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://workbench.haotushow.com/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "cookie-token"})
	if got := AuthTokenFromRequest(request); got != "cookie-token" {
		t.Fatalf("AuthTokenFromRequest = %q, want cookie-token", got)
	}
}
