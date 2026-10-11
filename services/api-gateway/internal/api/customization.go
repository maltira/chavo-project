package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/proto/avatarshape"
	"github.com/maltira/chavo-project-backend/proto/status"
)

type statusCatalogJSON struct {
	Icons     []string `json:"icons"`
	Durations []string `json:"durations"`
}

type customizationJSON struct {
	Status       statusCatalogJSON `json:"status"`
	AvatarShapes []string          `json:"avatar_shapes"`
}

func (h *Handler) customization(c *gin.Context) {
	durations := status.Durations()
	keys := make([]string, len(durations))
	for i, d := range durations {
		keys[i] = d.Key
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, customizationJSON{
		Status:       statusCatalogJSON{Icons: status.Icons(), Durations: keys},
		AvatarShapes: avatarshape.Shapes(),
	})
}
