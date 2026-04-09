package cors

import "github.com/gin-gonic/gin"

type Config struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
}

func New(config Config) gin.HandlerFunc {
	return func(c *gin.Context) {}
}
