package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"kegenbao/internal/config"
	"kegenbao/internal/database"
	"kegenbao/internal/middleware"
	"kegenbao/internal/models"
)

type AuthHandler struct{}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
}

// LoginRequest represents login request
type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请求参数错误"))
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := database.GetDB().Where("phone = ?", req.Phone).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "手机号已被注册"))
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "服务器错误"))
		return
	}

	// Create user
	user := models.User{
		Phone:        req.Phone,
		PasswordHash: string(hashedPassword),
		Nickname:     req.Nickname,
	}

	if err := database.GetDB().Create(&user).Error; err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "服务器错误"))
		return
	}

	// Generate token
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "生成token失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(LoginResponse{
		Token: token,
		User:  user,
	}))
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请求参数错误"))
		return
	}

	// Find user
	var user models.User
	if err := database.GetDB().Where("phone = ?", req.Phone).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "手机号或密码错误"))
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "手机号或密码错误"))
		return
	}

	// Generate token
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "生成token失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(LoginResponse{
		Token: token,
		User:  user,
	}))
}

// GetCurrentUser returns current user info
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := database.GetDB().First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(user))
}

func generateToken(userID uint) (string, error) {
	claims := middleware.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(config.AppConfig.GetJWTExpiration()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}