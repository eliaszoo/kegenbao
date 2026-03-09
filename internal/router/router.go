package router

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"kegenbao/internal/handlers"
	"kegenbao/internal/middleware"
)

func SetupRouter(frontendPath string) *gin.Engine {
	r := gin.Default()

	// Set up HTML templates for serving index.html for any non-API route
	r.SetHTMLTemplate(template.Must(template.New("index").ParseFiles(frontendPath + "/kegenbao.html")))

	// Serve static files from frontend directory
	r.Static("/static", frontendPath)
	r.StaticFile("/", frontendPath+"/kegenbao.html")
	r.StaticFile("/index.html", frontendPath+"/kegenbao.html")

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Auth routes (no auth required)
		authHandler := &handlers.AuthHandler{}
		v1.POST("/auth/register", authHandler.Register)
		v1.POST("/auth/login", authHandler.Login)

		// WeChat OAuth routes
		wechatHandler := &handlers.WechatHandler{}
		v1.GET("/auth/wechat/url", wechatHandler.GetAuthURL)
		v1.GET("/auth/wechat/callback", wechatHandler.Callback)

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			// Auth
			protected.GET("/auth/me", authHandler.GetCurrentUser)

			// Customer routes
			customerHandler := &handlers.CustomerHandler{}
			protected.GET("/customers", customerHandler.ListCustomers)
			protected.POST("/customers", customerHandler.CreateCustomer)
			protected.GET("/customers/:id", customerHandler.GetCustomer)
			protected.PUT("/customers/:id", customerHandler.UpdateCustomer)
			protected.DELETE("/customers/:id", customerHandler.DeleteCustomer)

			// Follow-up records
			protected.GET("/customers/:id/records", customerHandler.ListRecords)
			protected.POST("/customers/:id/records", customerHandler.CreateRecord)
			protected.DELETE("/records/:id", customerHandler.DeleteRecord)

			// WeChat messages
			protected.GET("/customers/:id/wechat", customerHandler.ListWeChatMessages)
			protected.POST("/customers/:id/wechat", customerHandler.CreateWeChatMessage)
			protected.DELETE("/wechat/:id", customerHandler.DeleteWeChatMessage)

			// AI routes
			aiHandler := &handlers.AIHandler{}
			protected.POST("/ai/briefing", aiHandler.GetBriefing)
			protected.POST("/ai/suggest/:id", aiHandler.GetSuggestion)
		}
	}

	return r
}