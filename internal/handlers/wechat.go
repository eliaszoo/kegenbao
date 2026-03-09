package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"kegenbao/internal/config"
	"kegenbao/internal/database"
	"kegenbao/internal/models"
)

type WechatHandler struct{}

// WechatLoginResponse represents the WeChat login response
type WechatLoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// GetWechatAuthURL returns the WeChat OAuth URL
func (h *WechatHandler) GetAuthURL(c *gin.Context) {
	// Generate state for security
	state := fmt.Sprintf("%d", time.Now().Unix())

	// Save state to session/cookie (simplified with client-side storage)
	// In production, use server-side session

	cfg := config.AppConfig.WeChat
	if cfg.AppID == "" || cfg.RedirectURI == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "微信配置未完成"))
		return
	}

	redirectURI := url.QueryEscape(cfg.RedirectURI)
	authURL := fmt.Sprintf(
		"https://open.weixin.qq.com/connect/oauth2/authorize?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_userinfo#wechat_redirect",
		cfg.AppID, redirectURI,
	)

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]string{
		"auth_url": authURL,
		"state":    state,
	}))
}

// Callback handles WeChat OAuth callback
func (h *WechatHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "授权码无效"))
		return
	}

	// Get WeChat access token
	tokenData, err := h.getWechatAccessToken(code)
	if err != nil {
		log.Printf("WeChat get access token error: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "微信授权失败"))
		return
	}

	// Get user info from WeChat
	userInfo, err := h.getWechatUserInfo(tokenData["access_token"].(string), tokenData["openid"].(string))
	if err != nil {
		log.Printf("WeChat get user info error: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "获取用户信息失败"))
		return
	}

	openID := tokenData["openid"].(string)

	// Find or create user
	var user models.User
	result := database.GetDB().Where("wechat_openid = ?", openID).First(&user)

	if result.Error != nil {
		// Create new user
		nickname := userInfo["nickname"].(string)
		if nickname == "" {
			nickname = "微信用户"
		}

		// Handle nickname encoding issues
		nickname = filterEmoji(nickname)

		user = models.User{
			WechatOpenID: openID,
			Nickname:     nickname,
			Avatar:       userInfo["headimgurl"].(string),
			Phone:        "",
			PasswordHash: "",
		}

		if err := database.GetDB().Create(&user).Error; err != nil {
			log.Printf("Failed to create WeChat user: %v", err)
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "创建用户失败"))
			return
		}
		log.Printf("Created new WeChat user: %s", nickname)
	} else {
		// Update user info if changed
		updates := map[string]interface{}{
			"nickname": filterEmoji(userInfo["nickname"].(string)),
			"avatar":   userInfo["headimgurl"].(string),
		}
		database.GetDB().Model(&user).Updates(updates)
	}

	// Generate JWT token
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "生成token失败"))
		return
	}

	// Return the frontend URL with token
	frontendURL := fmt.Sprintf("/?wechat_token=%s&state=%s", url.QueryEscape(token), state)

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]string{
		"redirect_url": frontendURL,
		"token":        token,
	}))
}

func (h *WechatHandler) getWechatAccessToken(code string) (map[string]interface{}, error) {
	cfg := config.AppConfig.WeChat

	tokenURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		cfg.AppID, cfg.AppSecret, code,
	)

	resp, err := http.Get(tokenURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result["errcode"] != nil {
		return nil, fmt.Errorf("WeChat error: %v", result)
	}

	return result, nil
}

func (h *WechatHandler) getWechatUserInfo(accessToken, openID string) (map[string]interface{}, error) {
	userInfoURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN",
		accessToken, openID,
	)

	resp, err := http.Get(userInfoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result["errcode"] != nil {
		return nil, fmt.Errorf("WeChat error: %v", result)
	}

	return result, nil
}

// filterEmoji removes emoji characters that may cause issues
func filterEmoji(s string) string {
	result := ""
	for _, r := range s {
		if r < 0x10000 {
			result += string(r)
		}
	}
	return result
}