package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"kegenbao/internal/database"
	"kegenbao/internal/middleware"
	"kegenbao/internal/models"
)

type CustomerHandler struct{}

// CreateCustomerRequest represents customer creation request
type CreateCustomerRequest struct {
	Name    string `json:"name" binding:"required"`
	Industry string `json:"industry"`
	Phone   string `json:"phone"`
	Temp    string `json:"temp"`
}

// UpdateCustomerRequest represents customer update request
type UpdateCustomerRequest struct {
	Name     string `json:"name"`
	Industry string `json:"industry"`
	Phone    string `json:"phone"`
	Temp     string `json:"temp"`
	Notes    string `json:"notes"`
}

// CreateRecordRequest represents follow-up record creation request
type CreateRecordRequest struct {
	Note string `json:"note" binding:"required"`
}

// ListCustomers returns list of customers
func (h *CustomerHandler) ListCustomers(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Parse query parameters
	temp := c.Query("temp")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "created_at")

	db := database.GetDB().Where("user_id = ?", userID)

	// Filter by temp
	if temp != "" {
		db = db.Where("temp = ?", temp)
	}

	// Search by name or phone
	if search != "" {
		searchPattern := "%" + search + "%"
		db = db.Where("name LIKE ? OR phone LIKE ?", searchPattern, searchPattern)
	}

	// Sort
	orderClause := "created_at DESC"
	switch sort {
	case "name":
		orderClause = "name ASC"
	case "temp":
		orderClause = "temp DESC, last_contact ASC"
	case "last_contact":
		orderClause = "last_contact ASC"
	}
	db = db.Order(orderClause)

	var customers []models.Customer
	if err := db.Find(&customers).Error; err != nil {
		log.Printf("Failed to list customers: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "获取客户列表失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(customers))
}

// CreateCustomer creates a new customer
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请求参数错误"))
		return
	}

	temp := req.Temp
	if temp == "" {
		temp = "温"
	}

	customer := models.Customer{
		UserID:      userID,
		Name:        req.Name,
		Industry:    req.Industry,
		Phone:       req.Phone,
		Temp:        temp,
		LastContact: nil,
		Notes:       "",
	}

	if err := database.GetDB().Create(&customer).Error; err != nil {
		log.Printf("Failed to create customer: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "创建客户失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(customer))
}

// GetCustomer returns a single customer with records
func (h *CustomerHandler) GetCustomer(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	// Get follow-up records
	var records []models.FollowUpRecord
	database.GetDB().Where("customer_id = ?", customerID).Order("created_at DESC").Find(&records)

	type CustomerWithRecords struct {
		models.Customer
		Records []models.FollowUpRecord `json:"records"`
	}

	c.JSON(http.StatusOK, models.SuccessResponse(CustomerWithRecords{
		Customer: customer,
		Records:  records,
	}))
}

// UpdateCustomer updates a customer
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	var req UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请求参数错误"))
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Industry != "" {
		updates["industry"] = req.Industry
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.Temp != "" {
		updates["temp"] = req.Temp
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}

	if err := database.GetDB().Model(&customer).Updates(updates).Error; err != nil {
		log.Printf("Failed to update customer: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "更新客户失败"))
		return
	}

	// Reload customer
	database.GetDB().First(&customer, customerID)

	c.JSON(http.StatusOK, models.SuccessResponse(customer))
}

// DeleteCustomer deletes a customer (soft delete)
func (h *CustomerHandler) DeleteCustomer(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	result := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).Delete(&models.Customer{})
	if result.Error != nil {
		log.Printf("Failed to delete customer: %v", result.Error)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "删除客户失败"))
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// ListRecords returns follow-up records for a customer
func (h *CustomerHandler) ListRecords(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	// Verify customer belongs to user
	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	var records []models.FollowUpRecord
	database.GetDB().Where("customer_id = ?", customerID).Order("created_at DESC").Find(&records)

	c.JSON(http.StatusOK, models.SuccessResponse(records))
}

// CreateRecord creates a follow-up record
func (h *CustomerHandler) CreateRecord(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	// Verify customer belongs to user
	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	var req CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请求参数错误"))
		return
	}

	now := time.Now()
	record := models.FollowUpRecord{
		CustomerID:  uint(customerID),
		UserID:      userID,
		Note:        req.Note,
		ContactedAt: now,
	}

	if err := database.GetDB().Create(&record).Error; err != nil {
		log.Printf("Failed to create record: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "创建跟进记录失败"))
		return
	}

	// Update customer's last contact time and notes
	database.GetDB().Model(&customer).Updates(map[string]interface{}{
		"last_contact": now,
		"notes":        req.Note,
	})

	c.JSON(http.StatusOK, models.SuccessResponse(record))
}

// DeleteRecord deletes a follow-up record
func (h *CustomerHandler) DeleteRecord(c *gin.Context) {
	userID := middleware.GetUserID(c)
	recordID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的记录ID"))
		return
	}

	// Verify record belongs to user
	var record models.FollowUpRecord
	if err := database.GetDB().Where("id = ? AND user_id = ?", recordID, userID).First(&record).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "记录不存在"))
		return
	}

	if err := database.GetDB().Delete(&record).Error; err != nil {
		log.Printf("Failed to delete record: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "删除记录失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}

// WeChatMessage handlers

// CreateWeChatMessage creates a wechat message
func (h *CustomerHandler) CreateWeChatMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	// Verify customer belongs to user
	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请求参数错误"))
		return
	}

	msg := models.WeChatMessage{
		CustomerID: uint(customerID),
		UserID:     userID,
		Content:    req.Content,
		MessageAt:  time.Now(),
	}

	if err := database.GetDB().Create(&msg).Error; err != nil {
		log.Printf("Failed to create wechat message: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "导入聊天记录失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(msg))
}

// ListWeChatMessages returns wechat messages for a customer
func (h *CustomerHandler) ListWeChatMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	customerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的客户ID"))
		return
	}

	// Verify customer belongs to user
	var customer models.Customer
	if err := database.GetDB().Where("id = ? AND user_id = ?", customerID, userID).First(&customer).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "客户不存在"))
		return
	}

	var messages []models.WeChatMessage
	database.GetDB().Where("customer_id = ?", customerID).Order("message_at DESC").Find(&messages)

	c.JSON(http.StatusOK, models.SuccessResponse(messages))
}

// DeleteWeChatMessage deletes a wechat message
func (h *CustomerHandler) DeleteWeChatMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	messageID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "无效的消息ID"))
		return
	}

	// Verify message belongs to user
	var msg models.WeChatMessage
	if err := database.GetDB().Where("id = ? AND user_id = ?", messageID, userID).First(&msg).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "消息不存在"))
		return
	}

	if err := database.GetDB().Delete(&msg).Error; err != nil {
		log.Printf("Failed to delete wechat message: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "删除聊天记录失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil))
}