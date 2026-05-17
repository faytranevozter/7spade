package handler

import "github.com/gin-gonic/gin"

func JSONError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
