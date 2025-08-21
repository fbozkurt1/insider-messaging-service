package api

import (
	"insider-message-service/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(h *Handler) *gin.Engine {

	router := gin.Default()
	docs.SwaggerInfo.BasePath = "/api"

	apiRoutes := router.Group("/api/messages")
	{
		apiRoutes.PUT("/toggle-job", h.toggleSendingMessagesJobHandler)
		apiRoutes.GET("/sent", h.getSentMessagesHandler)
		apiRoutes.POST("/", h.createMessageHandler)
	}
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
