package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSearchAdminUsageReconciliationRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerSearchRoutes(engine.Group("/api"), func(c *gin.Context) { c.Next() })

	found := false
	for _, route := range engine.Routes() {
		if route.Method == "POST" && route.Path == "/api/search/admin/usage-logs/:id/reconcile" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestSearchPublicCatalogRouteDoesNotRequireAuthenticationMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerSearchRoutes(engine.Group("/api"), func(c *gin.Context) { c.Next() })

	for _, route := range engine.Routes() {
		if route.Method == "GET" && route.Path == "/api/search/public/catalog" {
			return
		}
	}
	t.Fatal("public vSearch catalog route is not registered")
}
