package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/maltira/chavo-project-backend/proto/avatarshape"
	"github.com/maltira/chavo-project-backend/proto/mute"
	"github.com/maltira/chavo-project-backend/proto/status"
)

type statusCatalogJSON struct {
	Icons     []string `json:"icons"`
	Durations []string `json:"durations"`
}

type customizationJSON struct {
	Status       statusCatalogJSON `json:"status"`
	AvatarShapes []string          `json:"avatar_shapes"`
	// MuteDurations - сроки режима «без звука» для чата
	MuteDurations []string `json:"mute_durations"`
}

func (h *Handler) customization(c *gin.Context) {
	durations := status.Durations()
	keys := make([]string, len(durations))
	for i, d := range durations {
		keys[i] = d.Key
	}
	muteDurations := mute.Durations()
	muteKeys := make([]string, len(muteDurations))
	for i, d := range muteDurations {
		muteKeys[i] = d.Key
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, customizationJSON{
		Status:        statusCatalogJSON{Icons: status.Icons(), Durations: keys},
		AvatarShapes:  avatarshape.Shapes(),
		MuteDurations: muteKeys,
	})
}
