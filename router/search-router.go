package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerSearchRoutes(apiRouter *gin.RouterGroup, anonymousRequestBodyLimit gin.HandlerFunc) {
	apiRouter.GET("/search/public/catalog", controller.GetPublicSearchCatalog)

	searchAgentKeyRoute := apiRouter.Group("/search/keys")
	searchAgentKeyRoute.Use(middleware.UserAuth())
	{
		searchAgentKeyRoute.GET("", controller.GetSearchAgentKeys)
		searchAgentKeyRoute.POST("", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.CreateSearchAgentKey)
		searchAgentKeyRoute.DELETE("/:id", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.RevokeSearchAgentKey)
	}

	searchUserRoute := apiRouter.Group("/search")
	searchUserRoute.Use(middleware.UserAuth())
	{
		searchUserRoute.GET("/catalog", controller.GetSearchCatalog)
		searchUserRoute.GET("/discover", controller.DiscoverSearchCapabilities)
		searchUserRoute.GET("/describe/:service_id", controller.DescribeSearchCapability)
		searchUserRoute.POST("/execute", middleware.CriticalRateLimit(), controller.ExecuteSearchCapability)
		searchUserRoute.GET("/logs", controller.GetSearchLogs)
		searchUserRoute.GET("/logs/stat", controller.GetSearchLogStat)
	}

	searchManagedAgentKeyRoute := apiRouter.Group("/search/admin/keys")
	searchManagedAgentKeyRoute.Use(middleware.UserAuth(), middleware.SearchAdminAuth())
	{
		searchManagedAgentKeyRoute.GET("", controller.AdminGetSearchAgentKeys)
		searchManagedAgentKeyRoute.POST("", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminCreateSearchAgentKey)
		searchManagedAgentKeyRoute.DELETE("/:id", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminRevokeSearchAgentKey)
	}

	searchAdminRoute := apiRouter.Group("/search/admin")
	searchAdminRoute.Use(middleware.RootAuth())
	{
		searchAdminRoute.GET("/upstream-accounts", controller.AdminListSearchUpstreamAccounts)
		searchAdminRoute.POST("/upstream-accounts", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminCreateSearchUpstreamAccount)
		searchAdminRoute.PATCH("/upstream-accounts/:id", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminUpdateSearchUpstreamAccount)
		searchAdminRoute.DELETE("/upstream-accounts/:id", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminDeleteSearchUpstreamAccount)
		searchAdminRoute.POST("/upstream-accounts/:id/test", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminTestSearchUpstreamAccount)
		searchAdminRoute.GET("/catalog", controller.AdminGetSearchCatalog)
		searchAdminRoute.POST("/catalog/sync", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminSyncSearchCatalog)
		searchAdminRoute.POST("/catalog/publish", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminPublishSearchCatalog)
		searchAdminRoute.PATCH("/catalog/:id", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminConfigureSearchCapability)
		searchAdminRoute.GET("/catalog/:id/grants", controller.AdminGetSearchCapabilityEnterpriseGrants)
		searchAdminRoute.PUT("/catalog/:id/grants", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminSetSearchCapabilityEnterpriseGrants)
		searchAdminRoute.GET("/usage-logs", controller.AdminGetSearchUsageLogs)
		searchAdminRoute.GET("/usage-logs/stat", controller.AdminGetSearchUsageStat)
		searchAdminRoute.GET("/usage-logs/export", controller.AdminExportSearchUsageLogs)
		searchAdminRoute.POST("/usage-logs/:id/reconcile", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AdminReconcileSearchUsage)
	}

}
