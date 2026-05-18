package router

import "github.com/gin-gonic/gin"

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.ForwardedByClientIP = true

	return r
}
