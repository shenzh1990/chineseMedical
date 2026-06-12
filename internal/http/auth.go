package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const sessionCookieName = "cm_session"

func sessionSecret() []byte {
	secret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if secret == "" {
		secret = "chinese-medical-dev-session-secret"
	}
	return []byte(secret)
}

func signSession(username string, expires int64) string {
	payload := fmt.Sprintf("%s|%d", username, expires)
	mac := hmac.New(sha256.New, sessionSecret())
	mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature
}

func parseSession(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	payload := string(payloadBytes)
	fields := strings.Split(payload, "|")
	if len(fields) != 2 {
		return "", false
	}
	expires, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return "", false
	}

	expected := signSession(fields[0], expires)
	if !hmac.Equal([]byte(expected), []byte(value)) {
		return "", false
	}
	return fields[0], true
}

func setSessionCookie(c *gin.Context, username string) {
	expires := time.Now().Add(24 * time.Hour).Unix()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signSession(username, expires),
		Path:     "/",
		Expires:  time.Unix(expires, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h Handler) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		username, ok := parseSession(cookie)
		if !ok {
			clearSessionCookie(c)
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		if _, err := h.users.GetByUsername(c.Request.Context(), username); err != nil {
			clearSessionCookie(c)
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		c.Set("username", username)
		c.Next()
	}
}
