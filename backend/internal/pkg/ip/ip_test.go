//go:build unit

package ip

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetTrustedClientIPUsesGinClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))

	r.GET("/t", func(c *gin.Context) {
		c.String(200, GetTrustedClientIP(c))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = "9.9.9.9:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "1.2.3.4")
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	r.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)
	require.Equal(t, "9.9.9.9", w.Body.String())
}

func TestCheckIPRestrictionWithCompiledRules(t *testing.T) {
	whitelist := CompileIPRules([]string{"10.0.0.0/8", "192.168.1.2"})
	blacklist := CompileIPRules([]string{"10.1.1.1"})

	allowed, reason := CheckIPRestrictionWithCompiledRules("10.2.3.4", whitelist, blacklist)
	require.True(t, allowed)
	require.Equal(t, "", reason)

	allowed, reason = CheckIPRestrictionWithCompiledRules("10.1.1.1", whitelist, blacklist)
	require.False(t, allowed)
	require.Equal(t, "access denied", reason)
}

func TestCheckIPRestrictionWithCompiledRules_InvalidWhitelistStillDenies(t *testing.T) {
	// 与旧实现保持一致：白名单有配置但全无效时，最终应拒绝访问。
	invalidWhitelist := CompileIPRules([]string{"not-a-valid-pattern"})
	allowed, reason := CheckIPRestrictionWithCompiledRules("8.8.8.8", invalidWhitelist, nil)
	require.False(t, allowed)
	require.Equal(t, "access denied", reason)
}

func TestGetSecurityClientIPNeverTrustsHeadersFromUntrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name           string
		trustForwarded bool
		want           string
	}{
		{name: "legacy toggle disabled", trustForwarded: false, want: "9.9.9.9"},
		{name: "legacy toggle enabled", trustForwarded: true, want: "9.9.9.9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			require.NoError(t, r.SetTrustedProxies(nil))
			r.GET("/t", func(c *gin.Context) {
				c.String(200, GetSecurityClientIP(c, tc.trustForwarded))
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/t", nil)
			req.RemoteAddr = "9.9.9.9:12345"
			req.Header.Set("X-Real-IP", "1.2.3.4")
			r.ServeHTTP(w, req)

			require.Equal(t, 200, w.Code)
			require.Equal(t, tc.want, w.Body.String())
		})
	}
}

func TestGetSecurityClientIPUsesConfiguredTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies([]string{"9.9.9.9"}))
	r.GET("/t", func(c *gin.Context) { c.String(200, GetSecurityClientIP(c, true)) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = "9.9.9.9:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.ServeHTTP(w, req)

	require.Equal(t, "1.2.3.4", w.Body.String())
}

// TestGetClientIPReadsForwardedHeaders 验证 GetClientIP 从 header 提取真实 IP(回归测试)。
// 场景:无 trusted_proxies 配置时,反代(Caddy)转发的 X-Forwarded-For 应被正确读取。
func TestGetClientIPReadsForwardedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))

	tests := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{name: "CF-Connecting-IP", header: "CF-Connecting-IP", value: "203.0.113.5", want: "203.0.113.5"},
		{name: "X-Real-IP", header: "X-Real-IP", value: "198.51.100.10", want: "198.51.100.10"},
		{name: "X-Forwarded-For single", header: "X-Forwarded-For", value: "203.0.113.5", want: "203.0.113.5"},
		{name: "X-Forwarded-For multi picks first public", header: "X-Forwarded-For", value: "172.16.0.1, 203.0.113.5", want: "203.0.113.5"},
	}

	r.GET("/t2", func(c *gin.Context) { c.String(200, GetClientIP(c)) })

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/t2", nil)
			req.RemoteAddr = "172.18.0.1:12345" // docker 内网 IP(模拟 Caddy)
			req.Header.Set(tc.header, tc.value)
			r.ServeHTTP(w, req)
			require.Equal(t, 200, w.Code)
			require.Equal(t, tc.want, w.Body.String(), "GetClientIP 应从 %s 提取真实 IP", tc.header)
		})
	}
}
