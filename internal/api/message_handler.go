package api

import (
	"context"
	"insider-message-service/internal/api/dto"
	"insider-message-service/internal/domain"
	"insider-message-service/internal/services"
	"insider-message-service/internal/worker"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	messageService services.MessageService
	jobManager     *worker.JobManager
	appCtx         context.Context
}

func NewHandler(messageService services.MessageService, jobManager *worker.JobManager, ctx context.Context) *Handler {
	return &Handler{
		messageService: messageService,
		jobManager:     jobManager,
		appCtx:         ctx,
	}
}

// toggleSendingMessagesJobHandler
// @Summary      Starts or stops sending messages job
// @Description  Toggles the sending messages job based on its current state.
// If it is running, it will be stopped; if it is stopped, it will be started.
// @Tags         Messages
// @Produce      json
// @Success      200  {object}  dto.MessageJobResponse
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /messages/toggle-job [put]
func (h *Handler) toggleSendingMessagesJobHandler(c *gin.Context) {
	var err error
	var response dto.MessageJobResponse

	if h.jobManager.IsRunning() {
		err = h.jobManager.Stop()
		response = dto.MessageJobResponse{Status: "stopped"}
	} else {
		err = h.jobManager.Start(h.appCtx)
		response = dto.MessageJobResponse{Status: "started"}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// getSentMessagesHandler
// @Param        page  query  int false "page number"
// @Param        pageSize query  int  false "size of page"
// @Summary      Gets sent messages
// @Description  Fetches all messages  with status "sent" by pagination.
// @Tags         Messages
// @Produce      json
// @Success      200  {array}   dto.MessagesResponse
// @Success      204
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /messages/sent [get]
func (h *Handler) getSentMessagesHandler(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page number"})
		return
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || pageSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page size"})
		return
	}

	sentMessages, totalCount, err := h.messageService.GetSentMessages(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected error occurred while fetching sent messages."})
		return
	}

	if len(sentMessages) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, toMessageResponseList(sentMessages, totalCount))
}

// createMessageHandler
// @Summary      Creates a new message
// @Description  Creates a new message in the database with 'pending' status using the provided content and phone number.
// @Tags         Messages
// @Accept       json
// @Produce      json
// @Param        message  body      dto.CreateMessageRequest  true  "Message Information"
// @Success      201      {object}  dto.MessageResponse
// @Failure      400  {object}  dto.ErrorResponse
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /messages [post]
func (h *Handler) createMessageHandler(c *gin.Context) {
	var req dto.CreateMessageRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request: " + err.Error()})
		return
	}

	newMessage, err := h.messageService.CreateMessage(c.Request.Context(), req.Content, req.RecipientPhoneNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unexpected error occurred while creating message."})
		return
	}

	response := toMessageResponse(*newMessage)

	c.JSON(http.StatusCreated, response)
}

func toMessageResponse(msg domain.Message) dto.MessageResponse {
	return dto.MessageResponse{
		ID:                   msg.ID,
		Content:              msg.Content,
		RecipientPhoneNumber: msg.RecipientPhoneNumber,
	}
}

func toMessageResponseList(messages []domain.Message, totalCount int64) dto.MessagesResponse {
	responseList := make([]dto.MessageResponse, len(messages))
	for i, msg := range messages {
		responseList[i] = toMessageResponse(msg)
	}

	return dto.MessagesResponse{
		Messages: responseList,
		Total:    totalCount,
	}
}
