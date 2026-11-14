package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerZhiqiAssetRoutes(group *gin.RouterGroup) {
	group.GET("/groups", controller.ListZhiqiAssets)
	group.POST("/groups", middleware.CriticalRateLimit(), controller.CreateZhiqiAsset)
	group.GET("/groups/:id", controller.GetZhiqiAsset)
	group.PUT("/groups/:id", middleware.CriticalRateLimit(), controller.MutateZhiqiAsset)
	group.DELETE("/groups/:id", middleware.CriticalRateLimit(), controller.MutateZhiqiAsset)
	group.GET("/assets", controller.ListZhiqiAssets)
	group.POST("/assets", middleware.CriticalRateLimit(), controller.CreateZhiqiAsset)
	group.GET("/assets/:id", controller.GetZhiqiAsset)
	group.PUT("/assets/:id", middleware.CriticalRateLimit(), controller.MutateZhiqiAsset)
	group.DELETE("/assets/:id", middleware.CriticalRateLimit(), controller.MutateZhiqiAsset)
	group.GET("/assets/:id/preview", controller.GetZhiqiAsset)
	group.POST("/uploads", middleware.CriticalRateLimit(), controller.CreateZhiqiAsset)
	group.POST("/uploads/:id/complete", middleware.CriticalRateLimit(), controller.MutateZhiqiAsset)
}
