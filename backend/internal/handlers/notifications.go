package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
)

// NotificationHandler holds dependencies for notification routes.
type NotificationHandler struct {
	db *gorm.DB
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// --- Input / Output types ---

type listNotificationsInput struct {
	Unread bool `query:"unread" doc:"When true, return only unread notifications"`
}

type listNotificationsOutput struct{ Body []models.Notification }

type markReadInput struct {
	ID uint `path:"id" doc:"Notification ID"`
}

type markReadOutput struct{ Body models.Notification }

type markAllReadOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

// --- Route registration ---

// RegisterRoutes registers all notification routes on the given huma API.
func (h *NotificationHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-notifications",
		Method:      "GET",
		Path:        "/notifications",
		Tags:        []string{"notifications"},
		Summary:     "List notifications for the authenticated user",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listNotifications)

	// /notifications/read-all must be registered before /notifications/{id}/read
	// so the literal segment takes precedence over the wildcard.
	huma.Register(api, huma.Operation{
		OperationID: "mark-all-read",
		Method:      "PATCH",
		Path:        "/notifications/read-all",
		Tags:        []string{"notifications"},
		Summary:     "Mark all notifications as read",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.markAllRead)

	huma.Register(api, huma.Operation{
		OperationID: "mark-notification-read",
		Method:      "PATCH",
		Path:        "/notifications/{id}/read",
		Tags:        []string{"notifications"},
		Summary:     "Mark a single notification as read",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.markRead)
}

// --- Handlers ---

func (h *NotificationHandler) listNotifications(ctx context.Context, input *listNotificationsInput) (*listNotificationsOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	tx := h.db.Where("recipient_id = ?", userID).Order("created_at DESC")
	if input.Unread {
		tx = tx.Where("read = ?", false)
	}

	var notifications []models.Notification
	if err := tx.Find(&notifications).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not fetch notifications")
	}

	return &listNotificationsOutput{Body: notifications}, nil
}

func (h *NotificationHandler) markRead(ctx context.Context, input *markReadInput) (*markReadOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var n models.Notification
	if err := h.db.First(&n, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("notification not found")
	}
	if n.RecipientID != userID {
		return nil, huma.Error403Forbidden("access denied")
	}

	n.Read = true
	if err := h.db.Save(&n).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not update notification")
	}

	return &markReadOutput{Body: n}, nil
}

func (h *NotificationHandler) markAllRead(ctx context.Context, _ *struct{}) (*markAllReadOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if err := h.db.Model(&models.Notification{}).
		Where("recipient_id = ? AND read = ?", userID, false).
		Update("read", true).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not update notifications")
	}

	out := &markAllReadOutput{}
	out.Body.Message = "all notifications marked as read"
	return out, nil
}
