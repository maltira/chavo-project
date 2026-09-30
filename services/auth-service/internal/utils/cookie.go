package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetAuthCookies(c *gin.Context, refreshToken string, maxAge int) {
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", refreshToken, maxAge, "/", "", secure, true)
}

func ClearAuthCookies(c *gin.Context) {
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", "", -1, "/", "", secure, true)
}
