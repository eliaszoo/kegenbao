package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kegenbao/internal/database"
	"kegenbao/internal/middleware"
	"kegenbao/internal/models"
)

type AIHandler struct{}

// BriefingResponse represents the AI briefing response
type BriefingResponse struct {
	TopCustomers []TopCustomer `json:"top_customers"`
}

// TopCustomer represents a recommended customer
type TopCustomer struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Temp        string `json:"temp"`
	Reason      string `json:"reason"`
	Opener      string `json:"opener"`
	DaysSince   int    `json:"days_since_contact"`
}

// SuggestionResponse represents the AI suggestion response
type SuggestionResponse struct {
	Analysis string `json:"analysis"`
	Opener   string `json:"opener"`
	WinRate  int    `json:"win_rate"` // 成单概率 0-100
}

// CustomerInfo represents customer info for AI processing
type CustomerInfo struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Industry    string     `json:"industry"`
	Phone       string     `json:"phone"`
	Temp        string     `json:"temp"`
	LastContact *time.Time `json:"last_contact"`
	Notes       string     `json:"notes"`
	DaysSince   int        `json:"days_since_contact"`
}

// GetBriefing returns today's AI briefing
func (h *AIHandler) GetBriefing(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Get all customers with their latest record
	var customers []models.Customer
	if err := database.GetDB().Where("user_id = ?", userID).Order("created_at DESC").Find(&customers).Error; err != nil {
		log.Printf("Failed to get customers: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "获取客户列表失败"))
		return
	}

	if len(customers) == 0 {
		c.JSON(http.StatusOK, models.SuccessResponse(BriefingResponse{
			TopCustomers: []TopCustomer{},
		}))
		return
	}

	// Build customer info with days since contact
	customerInfos := make([]CustomerInfo, len(customers))
	now := time.Now()

	for i, customer := range customers {
		daysSince := 0
		if customer.LastContact != nil {
			daysSince = int(now.Sub(*customer.LastContact).Hours() / 24)
		}
		customerInfos[i] = CustomerInfo{
			ID:          customer.ID,
			Name:        customer.Name,
			Industry:    customer.Industry,
			Phone:       customer.Phone,
			Temp:        customer.Temp,
			LastContact: customer.LastContact,
			Notes:       customer.Notes,
			DaysSince:   daysSince,
		}
	}

	// Build prompt
	prompt := buildBriefingPrompt(customerInfos)

	// Call AI
	result, err := callAI(prompt)
	if err != nil {
		log.Printf("AI briefing error: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "AI生成简报失败"))
		return
	}

	// Parse result
	var briefing BriefingResponse
	if err := json.Unmarshal([]byte(result), &briefing); err != nil {
		// If parsing fails, return raw result
		log.Printf("Failed to parse AI response: %v", err)
		c.JSON(http.StatusOK, models.SuccessResponse(map[string]string{
			"raw": result,
		}))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(briefing))
}

// GetSuggestion returns AI suggestion for a single customer
func (h *AIHandler) GetSuggestion(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	// Get customer
	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	// Get latest record
	var latestRecord models.FollowUpRecord
	database.GetDB().Where("customer_id = ?", customerID).Order("created_at DESC").First(&latestRecord)

	// Get wechat messages
	var wechatMessages []models.WeChatMessage
	database.GetDB().Where("customer_id = ?", customerID).Order("message_at DESC").Find(&wechatMessages)

	// Build prompt
	prompt := buildSuggestionPrompt(customer, latestRecord, wechatMessages)

	// Call AI
	result, err := callAI(prompt)
	if err != nil {
		log.Printf("AI suggestion error: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "AI生成建议失败"))
		return
	}

	// Parse result
	var suggestion SuggestionResponse
	if err := json.Unmarshal([]byte(result), &suggestion); err != nil {
		log.Printf("Failed to parse AI response: %v", err)
		c.JSON(http.StatusOK, models.SuccessResponse(map[string]string{
			"raw": result,
		}))
		return
	}

	// Save win_rate to customer
	if suggestion.WinRate > 0 {
		database.GetDB().Model(&customer).Update("win_rate", suggestion.WinRate)
	}

	c.JSON(http.StatusOK, models.SuccessResponse(suggestion))
}

func buildBriefingPrompt(customers []CustomerInfo) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业销售顾问，帮助传统行业个人销售（建材、装修、保险、医疗器械等）提升客户跟进效率。\n\n")
	sb.WriteString("请分析以下客户列表，挑选出今天最需要联系的3位客户，并给出选择理由和开场白建议。\n\n")
	sb.WriteString("客户列表：\n")

	for _, c := range customers {
		sb.WriteString(fmt.Sprintf("- 客户%d: %s, 行业: %s, 电话: %s, 温度: %s, 距上次联系: %d天, 备注: %s\n",
			c.ID, c.Name, c.Industry, c.Phone, c.Temp, c.DaysSince, c.Notes))
	}

	sb.WriteString("\n请返回JSON格式结果：\n")
	sb.WriteString(`{
  "top_customers": [
    {"id": 1, "name": "客户名", "temp": "热", "reason": "选择理由", "opener": "开场白建议", "days_since_contact": 5},
    ...
  ]
}
注意：
1. 只返回JSON，不要其他文字
2. id必须是客户ID
3. days_since_contact是距今天数
4. 选择理由要简洁，20字以内
5. 开场白要自然、亲切，30字以内
`)

	return sb.String()
}

func buildSuggestionPrompt(customer models.Customer, record models.FollowUpRecord, wechatMessages []models.WeChatMessage) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业销售顾问，帮助传统行业个人销售提升客户跟进效率。\n\n")

	daysSince := 0
	if customer.LastContact != nil {
		daysSince = int(time.Now().Sub(*customer.LastContact).Hours() / 24)
	}

	sb.WriteString(fmt.Sprintf("请为以下客户生成分析和建议：\n"))
	sb.WriteString(fmt.Sprintf("- 客户姓名: %s\n", customer.Name))
	sb.WriteString(fmt.Sprintf("- 行业: %s\n", customer.Industry))
	sb.WriteString(fmt.Sprintf("- 电话: %s\n", customer.Phone))
	sb.WriteString(fmt.Sprintf("- 意向温度: %s\n", customer.Temp))
	sb.WriteString(fmt.Sprintf("- 距上次联系: %d天\n", daysSince))
	sb.WriteString(fmt.Sprintf("- 最新备注: %s\n", customer.Notes))

	if record.ID != 0 {
		sb.WriteString(fmt.Sprintf("- 最新跟进记录: %s\n", record.Note))
	}

	// Add wechat messages
	if len(wechatMessages) > 0 {
		sb.WriteString("\n- 微信聊天记录:\n")
		for i, msg := range wechatMessages {
			if i >= 5 { // Only show latest 5 messages
				break
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", msg.MessageAt.Format("2006-01-02 15:04"), msg.Content))
		}
	}

	sb.WriteString("\n请返回JSON格式结果：\n")
	sb.WriteString(`{
  "analysis": "客户意向分析，50字以内",
  "opener": "开场白建议，自然亲切，30字以内",
  "win_rate": 85
}
注意：
1. 只返回JSON，不要其他文字
2. win_rate 是成单概率，0-100 的整数
3. 分析要基于微信聊天记录和跟进记录综合判断
`)

	return sb.String()
}

// callAI calls the configured AI API
func callAI(prompt string) (string, error) {
	// Check for OpenAI-compatible API first (for qwen/kimi/minimax)
	openAIEndpoint := os.Getenv("OPENAI_API_ENDPOINT")
	openAIKey := os.Getenv("OPENAI_API_KEY")

	if openAIEndpoint != "" && openAIKey != "" {
		return callOpenAICompatibleAPI(openAIEndpoint, openAIKey, prompt)
	}

	// Fall back to Anthropic
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey != "" {
		return callAnthropicAPI(anthropicKey, prompt)
	}

	return "", fmt.Errorf("no AI API configured")
}

// callOpenAICompatibleAPI calls OpenAI-compatible API (qwen/kimi/minimax)
func callOpenAICompatibleAPI(endpoint, apiKey, prompt string) (string, error) {
	model := os.Getenv("AI_MODEL")
	if model == "" {
		model = "minimax"
	}

	log.Printf("Calling AI API: endpoint=%s, model=%s", endpoint, model)

	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", endpoint+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("AI request failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("AI response status: %d", resp.StatusCode)

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode AI response: %v", err)
		return "", err
	}

	// Check for error in response
	if errMsg, ok := result["error"].(map[string]interface{}); ok {
		return "", fmt.Errorf("AI API error: %v", errMsg)
	}

	// Parse response
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					// Remove markdown code block wrapper (```json ... ```)
					cleanContent := strings.TrimSpace(content)
					cleanContent = strings.TrimPrefix(cleanContent, "```json")
					cleanContent = strings.TrimPrefix(cleanContent, "```")
					cleanContent = strings.TrimSuffix(cleanContent, "```")
					cleanContent = strings.TrimSpace(cleanContent)
					return cleanContent, nil
				}
			}
		}
	}

	return "", fmt.Errorf("failed to parse AI response: %v", result)
}

// callAnthropicAPI calls Anthropic Claude API
func callAnthropicAPI(apiKey, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1000,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if block, ok := content[0].(map[string]interface{}); ok {
			if text, ok := block["text"].(string); ok {
				return text, nil
			}
		}
	}

	return "", fmt.Errorf("failed to parse Anthropic response")
}