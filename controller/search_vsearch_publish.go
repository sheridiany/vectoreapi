package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/vsearch"
	"github.com/gin-gonic/gin"
)

type searchCatalogPublishRequest struct {
	ServiceIDs []string `json:"service_ids"`
	AccessMode string   `json:"access_mode"`
}

func AdminPublishSearchCatalog(c *gin.Context) {
	var request searchCatalogPublishRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeSearchCatalogPublishError(c, &vsearch.PublicError{
			Code: "CATALOG_PUBLISH_INVALID", Message: "vSearch catalog publish request is invalid", HTTPStatus: http.StatusBadRequest,
		})
		return
	}
	result, err := searchControlPlane.PublishCatalog(c.Request.Context(), vsearch.PublishCommand{
		ServiceIDs: request.ServiceIDs,
		AccessMode: request.AccessMode,
	})
	if err != nil {
		writeSearchCatalogPublishError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func writeSearchCatalogPublishError(c *gin.Context, err error) {
	var publicErr *vsearch.PublicError
	if errors.As(err, &publicErr) {
		c.JSON(publicErr.HTTPStatus, gin.H{
			"success": false,
			"code":    publicErr.Code,
			"message": publicErr.Message,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"code":    "VSEARCH_INTERNAL_ERROR",
		"message": "vSearch service is temporarily unavailable",
	})
}
