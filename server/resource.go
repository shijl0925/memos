package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/usememos/memos/api"
)

func (s *Server) registerResourceRoutes(g *gin.RouterGroup) {
	g.POST("/resource", func(c *gin.Context) {
		userID := getCurrentUserID(c)

		err := c.Request.ParseMultipartForm(64 << 20)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "Upload file overload max size", err)
			return
		}

		file, err := c.FormFile("file")
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "Upload file not found", err)
			return
		}

		filename := file.Filename
		filetype := file.Header.Get("Content-Type")
		size := file.Size
		src, err := file.Open()
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to open file", err)
			return
		}
		defer src.Close()

		fileBytes, err := ioutil.ReadAll(src)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to read file", err)
			return
		}

		resourceCreate := &api.ResourceCreate{
			Filename:  filename,
			Type:      filetype,
			Size:      size,
			Blob:      fileBytes,
			CreatorID: userID,
		}

		resource, err := s.Store.CreateResource(resourceCreate)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to create resource", err)
			return
		}
		writeJSON(c, resource)
	})

	g.GET("/resource", func(c *gin.Context) {
		userID := getCurrentUserID(c)
		resourceFind := &api.ResourceFind{
			CreatorID: &userID,
		}
		list, err := s.Store.FindResourceList(resourceFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch resource list", err)
			return
		}
		writeJSON(c, list)
	})

	g.GET("/resource/:resourceId", func(c *gin.Context) {
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
			return
		}

		userID := getCurrentUserID(c)
		resourceFind := &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
		}
		resource, err := s.Store.FindResource(resourceFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch resource", err)
			return
		}
		writeJSON(c, resource)
	})

	g.GET("/resource/:resourceId/blob", func(c *gin.Context) {
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
			return
		}

		userID := getCurrentUserID(c)
		resourceFind := &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
		}
		resource, err := s.Store.FindResource(resourceFind)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to fetch resource", err)
			return
		}

		c.Data(http.StatusOK, resource.Type, resource.Blob)
	})

	g.DELETE("/resource/:resourceId", func(c *gin.Context) {
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			abortWithError(c, http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
			return
		}

		resourceDelete := &api.ResourceDelete{
			ID: resourceID,
		}
		if err := s.Store.DeleteResource(resourceDelete); err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to delete resource", err)
			return
		}

		c.JSON(http.StatusOK, true)
	})
}
