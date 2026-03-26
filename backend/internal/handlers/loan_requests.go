package handlers

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
	"github.com/tanjd/bookshelf/internal/repository"
	"github.com/tanjd/bookshelf/internal/services"
)

// LoanRequestHandler holds dependencies for loan-request routes.
type LoanRequestHandler struct {
	copies   repository.CopyRepository
	loanReqs repository.LoanRequestRepository
	admin    repository.AdminRepository
	users    repository.UserRepository
	workflow *services.LoanWorkflow
}

// NewLoanRequestHandler creates a new LoanRequestHandler.
func NewLoanRequestHandler(
	copies repository.CopyRepository,
	loanReqs repository.LoanRequestRepository,
	admin repository.AdminRepository,
	users repository.UserRepository,
	workflow *services.LoanWorkflow,
) *LoanRequestHandler {
	return &LoanRequestHandler{copies: copies, loanReqs: loanReqs, admin: admin, users: users, workflow: workflow}
}

// --- Input / Output types ---

type createLoanRequestInput struct {
	Body struct {
		CopyID             uint    `json:"copy_id" required:"true" minimum:"1" doc:"ID of the copy to borrow"`
		Message            string  `json:"message,omitempty" maxLength:"500" doc:"Optional message to the owner"`
		ExpectedReturnDate *string `json:"expected_return_date,omitempty" doc:"Expected return date (YYYY-MM-DD), required when copy has return_date_required"`
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
	ID                 uint                    `json:"id"`
	CopyID             uint                    `json:"copy_id"`
	BorrowerID         uint                    `json:"borrower_id"`
	Message            string                  `json:"message"`
	Status             string                  `json:"status"`
	RequestedAt        time.Time               `json:"requested_at"`
	RespondedAt        *time.Time              `json:"responded_at"`
	LoanedAt           *time.Time              `json:"loaned_at"`
	ReturnedAt         *time.Time              `json:"returned_at"`
	ExpectedReturnDate *time.Time              `json:"expected_return_date,omitempty"`
	Copy               loanRequestCopyResponse `json:"copy"`
	Borrower           safeUser                `json:"borrower"`
}

type getLoanRequestOutput struct{ Body getLoanRequestBody }

type listLoanRequestsInput struct {
	CopyID uint `query:"copy_id" minimum:"1" doc:"Copy ID to list requests for (owner only)"`
}

type listLoanRequestsOutput struct{ Body []getLoanRequestBody }

type listMineInput struct {
	Page     int `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 20)"`
}

type listMineOutput struct {
	Body struct {
		Items      []getLoanRequestBody `json:"items"`
		Total      int64                `json:"total"`
		Page       int                  `json:"page"`
		PageSize   int                  `json:"page_size"`
		TotalPages int                  `json:"total_pages"`
	}
}

type updateLoanRequestInput struct {
	ID   uint `path:"id" doc:"Loan request ID"`
	Body struct {
		Status       string `json:"status" required:"true" doc:"New status: accepted, rejected, returned, or cancelled"`
		NewCondition string `json:"new_condition,omitempty" doc:"Updated copy condition on return: good, fair, or worn"`
	}
}

type updateLoanRequestOutput struct{ Body models.LoanRequest }

// --- Route registration ---

// RegisterRoutes registers all loan-request routes on the given huma API.
func (h *LoanRequestHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-my-loan-requests",
		Method:      "GET",
		Path:        "/loan-requests/mine",
		Tags:        []string{"loan-requests"},
		Summary:     "List all loan requests made by the authenticated user (paginated)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMine)

	huma.Register(api, huma.Operation{
		OperationID: "list-loan-requests",
		Method:      "GET",
		Path:        "/loan-requests",
		Tags:        []string{"loan-requests"},
		Summary:     "List loan requests for a copy (owner only)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listLoanRequests)

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

func (h *LoanRequestHandler) listMine(ctx context.Context, input *listMineInput) (*listMineOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := h.loanReqs.ListByBorrowerIDPaginated(callerID, page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch loan requests")
	}

	bodies := make([]getLoanRequestBody, len(result.Items))
	for i, lr := range result.Items {
		borrowerResp := safeUser{ID: lr.Borrower.ID, Name: lr.Borrower.Name}
		ownerResp := safeUser{ID: lr.Copy.Owner.ID, Name: lr.Copy.Owner.Name}
		if lr.Status == "accepted" {
			borrowerResp.Email = lr.Borrower.Email
			borrowerResp.Phone = lr.Borrower.Phone
			ownerResp.Email = lr.Copy.Owner.Email
			ownerResp.Phone = lr.Copy.Owner.Phone
		}
		bodies[i] = getLoanRequestBody{
			ID:                 lr.ID,
			CopyID:             lr.CopyID,
			BorrowerID:         lr.BorrowerID,
			Message:            lr.Message,
			Status:             lr.Status,
			RequestedAt:        lr.RequestedAt,
			RespondedAt:        lr.RespondedAt,
			LoanedAt:           lr.LoanedAt,
			ReturnedAt:         lr.ReturnedAt,
			ExpectedReturnDate: lr.ExpectedReturnDate,
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
	}

	var out listMineOutput
	out.Body.Items = bodies
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *LoanRequestHandler) listLoanRequests(ctx context.Context, input *listLoanRequestsInput) (*listLoanRequestsOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.copies.GetByID(input.CopyID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("copy not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch copy")
	}
	if bookCopy.OwnerID != callerID {
		return nil, huma.Error403Forbidden("only the copy owner can list requests")
	}

	requests, err := h.loanReqs.ListByCopyID(input.CopyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch loan requests")
	}

	bodies := make([]getLoanRequestBody, len(requests))
	for i, lr := range requests {
		borrowerResp := safeUser{ID: lr.Borrower.ID, Name: lr.Borrower.Name}
		ownerResp := safeUser{ID: lr.Copy.Owner.ID, Name: lr.Copy.Owner.Name}
		if lr.Status == "accepted" {
			borrowerResp.Email = lr.Borrower.Email
			borrowerResp.Phone = lr.Borrower.Phone
			ownerResp.Email = lr.Copy.Owner.Email
			ownerResp.Phone = lr.Copy.Owner.Phone
		}
		bodies[i] = getLoanRequestBody{
			ID:                 lr.ID,
			CopyID:             lr.CopyID,
			BorrowerID:         lr.BorrowerID,
			Message:            lr.Message,
			Status:             lr.Status,
			RequestedAt:        lr.RequestedAt,
			RespondedAt:        lr.RespondedAt,
			LoanedAt:           lr.LoanedAt,
			ReturnedAt:         lr.ReturnedAt,
			ExpectedReturnDate: lr.ExpectedReturnDate,
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
	}
	return &listLoanRequestsOutput{Body: bodies}, nil
}

func (h *LoanRequestHandler) createLoanRequest(ctx context.Context, input *createLoanRequestInput) (*createLoanRequestOutput, error) {
	borrowerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.copies.GetByID(input.Body.CopyID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("copy not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch copy")
	}
	if bookCopy.OwnerID == borrowerID {
		return nil, huma.Error400BadRequest("you cannot request your own copy")
	}
	if bookCopy.Status != "available" {
		return nil, huma.Error400BadRequest("copy is not available")
	}

	// Load all settings in a single query for the eligibility checks below.
	allSettings, err := h.admin.GetSettings()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not load settings")
	}
	sm := make(map[string]string, len(allSettings))
	for _, s := range allSettings {
		sm[s.Key] = s.Value
	}

	// Enforce max active loans setting (0 = unlimited).
	if maxStr := sm["max_active_loans"]; maxStr != "" && maxStr != "0" {
		var maxLoans int64
		if _, scanErr := fmt.Sscanf(maxStr, "%d", &maxLoans); scanErr == nil && maxLoans > 0 {
			activeCount, countErr := h.loanReqs.CountActiveLoansByBorrower(borrowerID)
			if countErr == nil && activeCount >= maxLoans {
				return nil, huma.Error422UnprocessableEntity(
					fmt.Sprintf("you have reached the maximum of %d active loan(s)", maxLoans),
				)
			}
		}
	}

	// Load borrower for eligibility checks below.
	borrower, borrowerErr := h.users.FindByID(borrowerID)

	// Enforce require_verified_to_borrow.
	if sm["require_verified_to_borrow"] == "true" {
		if borrowerErr != nil || !borrower.Verified {
			return nil, huma.Error403Forbidden("a verified email is required to borrow books")
		}
	}

	// Enforce verification_requires_phone.
	if sm["verification_requires_phone"] == "true" {
		if borrowerErr != nil || borrower.Phone == "" {
			return nil, huma.Error403Forbidden("a phone number is required to borrow books")
		}
	}

	// Enforce verification_min_books_shared (0 = disabled).
	if minStr := sm["verification_min_books_shared"]; minStr != "" && minStr != "0" {
		var minBooks int64
		if _, scanErr := fmt.Sscanf(minStr, "%d", &minBooks); scanErr == nil && minBooks > 0 {
			sharedCount, countErr := h.copies.CountByOwnerID(borrowerID)
			if countErr == nil && sharedCount < minBooks {
				return nil, huma.Error403Forbidden(
					fmt.Sprintf("you must share at least %d book(s) before you can borrow", minBooks),
				)
			}
		}
	}

	// Validate return date requirement.
	if bookCopy.ReturnDateRequired && input.Body.ExpectedReturnDate == nil {
		return nil, huma.Error400BadRequest("return date is required by the sharer")
	}

	lr := models.LoanRequest{
		CopyID:      input.Body.CopyID,
		BorrowerID:  borrowerID,
		Message:     input.Body.Message,
		Status:      "pending",
		RequestedAt: time.Now(),
	}

	if input.Body.ExpectedReturnDate != nil {
		t, parseErr := time.Parse("2006-01-02", *input.Body.ExpectedReturnDate)
		if parseErr != nil {
			return nil, huma.Error400BadRequest("expected_return_date must be in YYYY-MM-DD format")
		}
		lr.ExpectedReturnDate = &t
	}

	// Atomically create the loan request and mark the copy as requested,
	// preventing a TOCTOU race where two concurrent requests both pass the
	// availability check above and result in two active loan requests.
	if err := h.loanReqs.CreateAndMarkRequested(&lr); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, huma.Error400BadRequest("copy is no longer available")
		}
		return nil, huma.Error500InternalServerError("could not create loan request")
	}

	// Load associations needed by the workflow.
	loaded, _ := h.loanReqs.GetByIDWithCopyOwnerAndBorrower(lr.ID)
	if loaded != nil {
		lr = *loaded
	}

	// Skip OnRequested when auto-approving to avoid sending a redundant
	// "someone wants to borrow your book" email that is immediately superseded.
	if !bookCopy.AutoApprove {
		if err := h.workflow.OnRequested(&lr); err != nil {
			log.Error().Err(err).Msg("workflow.OnRequested failed")
		}
	}

	// Auto-approve if enabled.
	if bookCopy.AutoApprove {
		now := time.Now()
		lr.Status = "accepted"
		lr.RespondedAt = &now
		if saveErr := h.loanReqs.Save(&lr); saveErr != nil {
			log.Error().Err(saveErr).Msg("auto-approve save failed")
		} else {
			if wErr := h.workflow.OnAccepted(&lr); wErr != nil {
				log.Error().Err(wErr).Msg("workflow.OnAccepted failed for auto-approve")
			}
			if reloaded, relErr := h.loanReqs.GetByIDWithCopyOwnerAndBorrower(lr.ID); relErr == nil {
				lr = *reloaded
			}
		}
	}

	return &createLoanRequestOutput{Body: lr}, nil
}

func (h *LoanRequestHandler) getLoanRequest(ctx context.Context, input *getLoanRequestInput) (*getLoanRequestOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	lr, err := h.loanReqs.GetByIDWithFullAssociations(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("loan request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch loan request")
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
		ID:                 lr.ID,
		CopyID:             lr.CopyID,
		BorrowerID:         lr.BorrowerID,
		Message:            lr.Message,
		Status:             lr.Status,
		RequestedAt:        lr.RequestedAt,
		RespondedAt:        lr.RespondedAt,
		LoanedAt:           lr.LoanedAt,
		ReturnedAt:         lr.ReturnedAt,
		ExpectedReturnDate: lr.ExpectedReturnDate,
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

	lr, err := h.loanReqs.GetByIDWithCopyAndBorrower(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("loan request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch loan request")
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
		// Update copy condition if provided.
		if cond := input.Body.NewCondition; cond != "" {
			allowed := map[string]bool{"good": true, "fair": true, "worn": true}
			if !allowed[cond] {
				return nil, huma.Error400BadRequest("new_condition must be good, fair, or worn")
			}
			lr.Copy.Condition = cond
			if saveErr := h.copies.Save(&lr.Copy); saveErr != nil {
				log.Error().Err(saveErr).Msg("failed to update copy condition on return")
			}
		}

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

	if err := h.loanReqs.Save(lr); err != nil {
		return nil, huma.Error500InternalServerError("could not update loan request")
	}

	// Run workflow side-effects (non-fatal).
	var workflowErr error
	switch lr.Status {
	case "accepted":
		workflowErr = h.workflow.OnAccepted(lr)
	case "rejected":
		workflowErr = h.workflow.OnRejected(lr)
	case "cancelled":
		workflowErr = h.workflow.OnCancelled(lr)
	case "returned":
		workflowErr = h.workflow.OnReturned(lr)
	}
	if workflowErr != nil {
		log.Error().Err(workflowErr).Str("status", lr.Status).Msg("workflow side-effect failed")
	}

	return &updateLoanRequestOutput{Body: *lr}, nil
}
