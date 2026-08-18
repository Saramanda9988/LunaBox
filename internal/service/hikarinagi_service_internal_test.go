package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestBuildHikarinagiAuthURLUsesPublicClientPKCE(t *testing.T) {
	session := &hikarinagiAuthSession{
		state:        "state-value",
		nonce:        "nonce-value",
		codeVerifier: "verifier-value",
		redirectURI:  hikarinagiOAuthRedirectURI,
	}
	authURL, err := url.Parse(buildHikarinagiAuthURL("public-client-id", session))
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	query := authURL.Query()
	if authURL.Scheme+"://"+authURL.Host+authURL.Path != hikarinagiOAuthAuthorizeURL {
		t.Fatalf("授权端点不符合预期: %s", authURL.String())
	}
	if query.Get("client_id") != "public-client-id" || query.Get("response_type") != "code" {
		t.Fatalf("public client 授权参数不完整: %s", authURL.RawQuery)
	}
	if query.Get("redirect_uri") != hikarinagiOAuthRedirectURI {
		t.Fatalf("回调地址不符合预期: %q", query.Get("redirect_uri"))
	}
	if query.Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE 方法不符合预期: %q", query.Get("code_challenge_method"))
	}
	digest := sha256.Sum256([]byte(session.codeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(digest[:])
	if query.Get("code_challenge") != expectedChallenge {
		t.Fatalf("PKCE challenge 不符合预期")
	}
	for _, scope := range []string{"openid", "catalog:full", "user:read", "status:write", "offline_access"} {
		if !strings.Contains(" "+query.Get("scope")+" ", " "+scope+" ") {
			t.Fatalf("授权请求缺少 scope %q: %q", scope, query.Get("scope"))
		}
	}
	if query.Get("prompt") != "consent" || query.Get("state") != session.state || query.Get("nonce") != session.nonce {
		t.Fatalf("授权请求缺少刷新令牌或防伪参数: %s", authURL.RawQuery)
	}
	if query.Has("client_secret") {
		t.Fatalf("public client 授权请求携带了 client_secret")
	}
}

func TestValidateHikarinagiIDTokenNonce(t *testing.T) {
	payload, err := json.Marshal(hikarinagiTokenClaims{Nonce: "expected-nonce"})
	if err != nil {
		t.Fatalf("编码测试声明失败: %v", err)
	}
	idToken := "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	if err := validateHikarinagiIDTokenNonce(idToken, "expected-nonce"); err != nil {
		t.Fatalf("有效 nonce 校验失败: %v", err)
	}
	if err := validateHikarinagiIDTokenNonce(idToken, "different-nonce"); err == nil {
		t.Fatalf("nonce 不一致时应返回错误")
	}
}

func TestResolveHikarinagiAssetURLUsesImageHost(t *testing.T) {
	const relativeURL = "images/ff29883b-acba-42c4-ab21-eccb0719a472.webp"
	const expectedURL = "https://imagesp.yurari.moe/images/ff29883b-acba-42c4-ab21-eccb0719a472.webp"
	if actualURL := resolveHikarinagiAssetURL(relativeURL); actualURL != expectedURL {
		t.Fatalf("Hikarinagi 图片地址解析错误: %q", actualURL)
	}

	const absoluteURL = "https://cdn.example.com/avatar.webp"
	if actualURL := resolveHikarinagiAssetURL(absoluteURL); actualURL != absoluteURL {
		t.Fatalf("Hikarinagi 绝对图片地址发生变化: %q", actualURL)
	}
}
