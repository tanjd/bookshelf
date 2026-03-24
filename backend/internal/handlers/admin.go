package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
	"github.com/tanjd/bookshelf/internal/repository"
)

// AdminHandler holds dependencies for admin routes.
type AdminHandler struct {
	admin repository.AdminRepository
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(admin repository.AdminRepository) *AdminHandler {
	return &AdminHandler{admin: admin}
}

// --- Input / Output types ---

type adminUsersInput struct {
	Page     int `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 50)"`
}

type adminUsersOutput struct {
	Body struct {
		Items      []models.User `json:"items"`
		Total      int64         `json:"total"`
		Page       int           `json:"page"`
		PageSize   int           `json:"page_size"`
		TotalPages int           `json:"total_pages"`
	}
}

type adminUserIDInput struct {
	ID uint `path:"id" doc:"User ID"`
}

type updateAdminUserInput struct {
	ID   uint `path:"id" doc:"User ID"`
	Body struct {
		Role     *string `json:"role,omitempty" doc:"Role: user or admin"`
		Verified *bool   `json:"verified,omitempty" doc:"Whether the user is verified"`
	}
}

type adminUserOutput struct {
	Body models.User
}

type adminSettingsOutput struct {
	Body []models.AppSetting
}

type updateSettingsInput struct {
	Body []struct {
		Key   string `json:"key" required:"true" doc:"Setting key"`
		Value string `json:"value" required:"true" doc:"Setting value"`
	}
}

// --- Route registration ---

// RegisterRoutes registers all admin routes on the given huma API.
func (h *AdminHandler) RegisterRoutes(api huma.API) {
	security := []map[string][]string{{"bearer": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-users",
		Method:      "GET",
		Path:        "/admin/users",
		Tags:        []string{"admin"},
		Summary:     "List all users",
		Security:    security,
	}, h.listUsers)

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-user",
		Method:      "PATCH",
		Path:        "/admin/users/{id}",
		Tags:        []string{"admin"},
		Summary:     "Update a user's role or verified status",
		Security:    security,
	}, h.updateUser)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-delete-user",
		Method:        "DELETE",
		Path:          "/admin/users/{id}",
		Tags:          []string{"admin"},
		Summary:       "Delete a user",
		Security:      security,
		DefaultStatus: 204,
	}, h.deleteUser)

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-settings",
		Method:      "GET",
		Path:        "/admin/settings",
		Tags:        []string{"admin"},
		Summary:     "Get all app settings",
		Security:    security,
	}, h.getSettings)

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-settings",
		Method:      "PATCH",
		Path:        "/admin/settings",
		Tags:        []string{"admin"},
		Summary:     "Upsert app settings",
		Security:    security,
	}, h.updateSettings)
}

// --- Handlers ---

func (h *AdminHandler) listUsers(ctx context.Context, input *adminUsersInput) (*adminUsersOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	result, err := h.admin.ListUsersPaginated(page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list users")
	}
	var out adminUsersOutput
	out.Body.Items = result.Items
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *AdminHandler) updateUser(ctx context.Context, input *updateAdminUserInput) (*adminUserOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	callerID, _ := middleware.GetRequiredUserID(ctx)

	user, err := h.admin.FindUserByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	if input.Body.Role != nil {
		// Prevent admins from demoting themselves
		if user.ID == callerID && *input.Body.Role != "admin" {
			return nil, huma.Error400BadRequest("cannot demote yourself")
		}
		// Prevent demoting the last admin
		if user.Role == "admin" && *input.Body.Role != "admin" {
			count, err := h.admin.CountByRole("admin")
			if err != nil {
				return nil, huma.Error500InternalServerError("could not check admin count")
			}
			if count <= 1 {
				return nil, huma.Error400BadRequest("cannot demote the last admin")
			}
		}
		user.Role = *input.Body.Role
	}
	if input.Body.Verified != nil {
		user.Verified = *input.Body.Verified
	}

	if err := h.admin.SaveUser(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &adminUserOutput{Body: *user}, nil
}

func (h *AdminHandler) deleteUser(ctx context.Context, input *adminUserIDInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	callerID, _ := middleware.GetRequiredUserID(ctx)
	if uint(input.ID) == callerID {
		return nil, huma.Error400BadRequest("cannot delete yourself")
	}

	// Prevent deleting the last admin
	target, err := h.admin.FindUserByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}
	if target.Role == "admin" {
		count, err := h.admin.CountByRole("admin")
		if err != nil {
			return nil, huma.Error500InternalServerError("could not check admin count")
		}
		if count <= 1 {
			return nil, huma.Error400BadRequest("cannot delete the last admin")
		}
	}

	if err := h.admin.DeleteUser(input.ID); err != nil {
		return nil, huma.Error500InternalServerError("could not delete user")
	}

	return nil, nil
}

func (h *AdminHandler) getSettings(ctx context.Context, _ *struct{}) (*adminSettingsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	settings, err := h.admin.GetSettings()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch settings")
	}

	return &adminSettingsOutput{Body: settings}, nil
}

func (h *AdminHandler) updateSettings(ctx context.Context, input *updateSettingsInput) (*adminSettingsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	for _, kv := range input.Body {
		if err := h.admin.UpsertSetting(kv.Key, kv.Value); err != nil {
			return nil, huma.Error500InternalServerError("could not save setting: " + kv.Key)
		}
	}

	settings, err := h.admin.GetSettings()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch settings")
	}

	return &adminSettingsOutput{Body: settings}, nil
}

// adminError maps middleware sentinel errors to appropriate huma errors.
func adminError(err error) error {
	if errors.Is(err, middleware.ErrUnauthorized) {
		return huma.Error401Unauthorized("authentication required")
	}
	return huma.Error403Forbidden("admin access required")
}
