package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseLimit(c *gin.Context, defaultLimit, maxLimit int) int {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit < 1 || limit > maxLimit {
		return defaultLimit
	}
	return limit
}

func parseOffset(c *gin.Context) int {
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}
