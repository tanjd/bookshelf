package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
	"github.com/tanjd/bookshelf/internal/services"
)

// LoanRequestHandler holds dependencies for loan-request routes.
type LoanRequestHandler struct {
	db       *gorm.DB
	workflow *services.LoanWorkflow
}

// NewLoanRequestHandler creates a new LoanRequestHandler.
func NewLoanRequestHandler(db *gorm.DB, workflow *services.LoanWorkflow) *LoanRequestHandler {
	return &LoanRequestHandler{db: db, workflow: workflow}
}

// --- Input / Output types ---

type createLoanRequestInput struct {
	Body struct {
		CopyID  uint   `json:"copy_id" required:"true" minimum:"1" doc:"ID of the copy to borrow"`
		Message string `json:"message,omitempty" maxLength:"500" doc:"Optional message to the owner"`
	}
}

type createLoanRequestOutput struct{ Body models.LoanRequest }

type getLoanRequestInput struct {
	ID uint `path:"id" doc:"Loan request ID"`
}

// safeUser redacts contact info when the loan has not yet been accepted.
type safeUser struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type loanRequestCopyResponse struct {
	ID        uint        `json:"id"`
	BookID    uint        `json:"book_id"`
	OwnerID   uint        `json:"owner_id"`
	Condition string      `json:"condition"`
	Notes     string      `json:"notes"`
	Status    string      `json:"status"`
	Book      models.Book `json:"book,omitempty"`
	Owner     safeUser    `json:"owner,omitempty"`
}

type getLoanRequestBody struct {
	ID          uint                    `json:"id"`
	CopyID      uint                    `json:"copy_id"`
	BorrowerID  uint                    `json:"borrower_id"`
	Message     string                  `json:"message"`
	Status      string                  `json:"status"`
	RequestedAt time.Time               `json:"requested_at"`
	RespondedAt *time.Time              `json:"responded_at"`
	LoanedAt    *time.Time              `json:"loaned_at"`
	ReturnedAt  *time.Time              `json:"returned_at"`
	Copy        loanRequestCopyResponse `json:"copy"`
	Borrower    safeUser                `json:"borrower"`
}

type getLoanRequestOutput struct{ Body getLoanRequestBody }

type updateLoanRequestInput struct {
	ID   uint `path:"id" doc:"Loan request ID"`
	Body struct {
		Status string `json:"status" required:"true" doc:"New status: accepted, rejected, returned, or cancelled"`
	}
}

type updateLoanRequestOutput struct{ Body models.LoanRequest }

// --- Route registration ---

// RegisterRoutes registers all loan-request routes on the given huma API.
func (h *LoanRequestHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-loan-request",
		Method:        "POST",
		Path:          "/loan-requests",
		Tags:          []string{"loan-requests"},
		Summary:       "Request to borrow a copy",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.createLoanRequest)

	huma.Register(api, huma.Operation{
		OperationID: "get-loan-request",
		Method:      "GET",
		Path:        "/loan-requests/{id}",
		Tags:        []string{"loan-requests"},
		Summary:     "Get a loan request (contact info revealed only when accepted)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.getLoanRequest)

	huma.Register(api, huma.Operation{
		OperationID: "update-loan-request",
		Method:      "PATCH",
		Path:        "/loan-requests/{id}",
		Tags:        []string{"loan-requests"},
		Summary:     "Update a loan request status (accept, reject, cancel, or mark returned)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateLoanRequest)
}

// --- Handlers ---

func (h *LoanRequestHandler) createLoanRequest(ctx context.Context, input *createLoanRequestInput) (*createLoanRequestOutput, error) {
	borrowerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var bookCopy models.Copy
	if err := h.db.First(&bookCopy, input.Body.CopyID).Error; err != nil {
		return nil, huma.Error404NotFound("copy not found")
	}
	if bookCopy.OwnerID == borrowerID {
		return nil, huma.Error400BadRequest("you cannot request your own copy")
	}
	if bookCopy.Status != "available" {
		return nil, huma.Error400BadRequest("copy is not available")
	}

	lr := models.LoanRequest{
		CopyID:      input.Body.CopyID,
		BorrowerID:  borrowerID,
		Message:     input.Body.Message,
		Status:      "pending",
		RequestedAt: time.Now(),
	}
	if err := h.db.Create(&lr).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not create loan request")
	}

	// Mark copy as requested.
	h.db.Model(&bookCopy).Update("status", "requested")

	// Load associations needed by the workflow.
	h.db.Preload("Copy.Owner").Preload("Borrower").First(&lr, lr.ID)

	if err := h.workflow.OnRequested(&lr); err != nil {
		slog.ErrorContext(ctx, "workflow.OnRequested failed", "error", err)
	}

	return &createLoanRequestOutput{Body: lr}, nil
}

func (h *LoanRequestHandler) getLoanRequest(ctx context.Context, input *getLoanRequestInput) (*getLoanRequestOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var lr models.LoanRequest
	if err := h.db.Preload("Copy.Book").Preload("Copy.Owner").Preload("Borrower").First(&lr, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("loan request not found")
	}

	ownerID := lr.Copy.OwnerID
	if lr.BorrowerID != callerID && ownerID != callerID {
		return nil, huma.Error403Forbidden("access denied")
	}

	showContact := lr.Status == "accepted" && (callerID == lr.BorrowerID || callerID == ownerID)

	borrowerResp := safeUser{ID: lr.Borrower.ID, Name: lr.Borrower.Name}
	ownerResp := safeUser{ID: lr.Copy.Owner.ID, Name: lr.Copy.Owner.Name}
	if showContact {
		borrowerResp.Email = lr.Borrower.Email
		borrowerResp.Phone = lr.Borrower.Phone
		ownerResp.Email = lr.Copy.Owner.Email
		ownerResp.Phone = lr.Copy.Owner.Phone
	}

	body := getLoanRequestBody{
		ID:          lr.ID,
		CopyID:      lr.CopyID,
		BorrowerID:  lr.BorrowerID,
		Message:     lr.Message,
		Status:      lr.Status,
		RequestedAt: lr.RequestedAt,
		RespondedAt: lr.RespondedAt,
		LoanedAt:    lr.LoanedAt,
		ReturnedAt:  lr.ReturnedAt,
		Copy: loanRequestCopyResponse{
			ID:        lr.Copy.ID,
			BookID:    lr.Copy.BookID,
			OwnerID:   lr.Copy.OwnerID,
			Condition: lr.Copy.Condition,
			Notes:     lr.Copy.Notes,
			Status:    lr.Copy.Status,
			Book:      lr.Copy.Book,
			Owner:     ownerResp,
		},
		Borrower: borrowerResp,
	}

	return &getLoanRequestOutput{Body: body}, nil
}

func (h *LoanRequestHandler) updateLoanRequest(ctx context.Context, input *updateLoanRequestInput) (*updateLoanRequestOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var lr models.LoanRequest
	if err := h.db.Preload("Copy").Preload("Borrower").First(&lr, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("loan request not found")
	}

	ownerID := lr.Copy.OwnerID
	now := time.Now()

	switch input.Body.Status {
	case "accepted", "rejected":
		if callerID != ownerID {
			return nil, huma.Error403Forbidden("only the copy owner can accept or reject")
		}
		if lr.Status != "pending" {
			return nil, huma.Error400BadRequest("can only accept/reject pending requests")
		}
		lr.Status = input.Body.Status
		lr.RespondedAt = &now

	case "returned":
		if callerID != ownerID {
			return nil, huma.Error403Forbidden("only the copy owner can mark as returned")
		}
		if lr.Status != "accepted" {
			return nil, huma.Error400BadRequest("can only mark accepted loans as returned")
		}
		lr.Status = "returned"
		lr.ReturnedAt = &now

	case "cancelled":
		if callerID != lr.BorrowerID {
			return nil, huma.Error403Forbidden("only the borrower can cancel")
		}
		if lr.Status != "pending" {
			return nil, huma.Error400BadRequest("can only cancel pending requests")
		}
		lr.Status = "cancelled"

	default:
		return nil, huma.Error400BadRequest("invalid status transition")
	}

	if err := h.db.Save(&lr).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not update loan request")
	}

	// Run workflow side-effects (non-fatal).
	var workflowErr error
	switch lr.Status {
	case "accepted":
		workflowErr = h.workflow.OnAccepted(&lr)
	case "rejected":
		workflowErr = h.workflow.OnRejected(&lr)
	case "cancelled":
		workflowErr = h.workflow.OnCancelled(&lr)
	case "returned":
		workflowErr = h.workflow.OnReturned(&lr)
	}
	if workflowErr != nil {
		slog.ErrorContext(ctx, "workflow side-effect failed", "status", lr.Status, "error", workflowErr)
	}

	return &updateLoanRequestOutput{Body: lr}, nil
}
