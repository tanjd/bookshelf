package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
)

// CopyHandler holds dependencies for copy routes.
type CopyHandler struct {
	db *gorm.DB
}

// NewCopyHandler creates a new CopyHandler.
func NewCopyHandler(db *gorm.DB) *CopyHandler {
	return &CopyHandler{db: db}
}

// --- Input / Output types ---

type createCopyInput struct {
	Body struct {
		BookID    uint   `json:"book_id" required:"true" minimum:"1" doc:"ID of the book"`
		Condition string `json:"condition,omitempty" doc:"Physical condition: good, fair, or worn"`
		Notes     string `json:"notes,omitempty" doc:"Optional notes visible to borrowers"`
	}
}

type createCopyOutput struct{ Body models.Copy }

type updateCopyInput struct {
	ID   uint `path:"id" doc:"Copy ID"`
	Body struct {
		Condition *string `json:"condition,omitempty" doc:"Physical condition: good, fair, or worn"`
		Notes     *string `json:"notes,omitempty" doc:"Notes visible to borrowers"`
		Status    *string `json:"status,omitempty" doc:"Status: available or unavailable"`
	}
}

type updateCopyOutput struct{ Body models.Copy }

type deleteCopyInput struct {
	ID uint `path:"id" doc:"Copy ID"`
}

// --- Route registration ---

// RegisterRoutes registers all copy routes on the given huma API.
func (h *CopyHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-copy",
		Method:        "POST",
		Path:          "/copies",
		Tags:          []string{"copies"},
		Summary:       "Add a physical copy of a book",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.createCopy)

	huma.Register(api, huma.Operation{
		OperationID: "update-copy",
		Method:      "PATCH",
		Path:        "/copies/{id}",
		Tags:        []string{"copies"},
		Summary:     "Update condition, notes, or status of a copy",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateCopy)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-copy",
		Method:        "DELETE",
		Path:          "/copies/{id}",
		Tags:          []string{"copies"},
		Summary:       "Delete a copy (only if not currently loaned or requested)",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 204,
	}, h.deleteCopy)
}

// --- Handlers ---

func (h *CopyHandler) createCopy(ctx context.Context, input *createCopyInput) (*createCopyOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy := models.Copy{
		BookID:    input.Body.BookID,
		OwnerID:   ownerID,
		Condition: input.Body.Condition,
		Notes:     input.Body.Notes,
		Status:    "available",
	}
	if err := h.db.Create(&bookCopy).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not create copy")
	}

	// Reload with associations.
	h.db.Preload("Book").Preload("Owner").First(&bookCopy, bookCopy.ID)
	return &createCopyOutput{Body: bookCopy}, nil
}

func (h *CopyHandler) updateCopy(ctx context.Context, input *updateCopyInput) (*updateCopyOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var bookCopy models.Copy
	if err := h.db.First(&bookCopy, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("copy not found")
	}
	if bookCopy.OwnerID != ownerID {
		return nil, huma.Error403Forbidden("you do not own this copy")
	}

	if input.Body.Condition != nil {
		bookCopy.Condition = *input.Body.Condition
	}
	if input.Body.Notes != nil {
		bookCopy.Notes = *input.Body.Notes
	}
	if input.Body.Status != nil {
		allowed := map[string]bool{"available": true, "unavailable": true}
		if !allowed[*input.Body.Status] {
			return nil, huma.Error400BadRequest("status must be 'available' or 'unavailable'")
		}
		bookCopy.Status = *input.Body.Status
	}

	if err := h.db.Save(&bookCopy).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not update copy")
	}

	h.db.Preload("Book").Preload("Owner").First(&bookCopy, bookCopy.ID)
	return &updateCopyOutput{Body: bookCopy}, nil
}

func (h *CopyHandler) deleteCopy(ctx context.Context, input *deleteCopyInput) (*struct{}, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var bookCopy models.Copy
	if err := h.db.First(&bookCopy, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("copy not found")
	}
	if bookCopy.OwnerID != ownerID {
		return nil, huma.Error403Forbidden("you do not own this copy")
	}
	if bookCopy.Status == "loaned" || bookCopy.Status == "requested" {
		return nil, huma.Error400BadRequest("cannot delete a copy that is loaned or requested")
	}

	if err := h.db.Delete(&bookCopy).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not delete copy")
	}

	return nil, nil
}
