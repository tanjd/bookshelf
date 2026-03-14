package services

import (
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/models"
)

// LoanWorkflow orchestrates side-effects (notifications, emails, copy-status
// updates) that occur at each stage of a loan request lifecycle.
type LoanWorkflow struct {
	db    *gorm.DB
	email *EmailService
}

// NewLoanWorkflow creates a new LoanWorkflow.
func NewLoanWorkflow(db *gorm.DB, email *EmailService) *LoanWorkflow {
	return &LoanWorkflow{db: db, email: email}
}

// OnRequested fires when a borrower creates a new loan request.
// It notifies the copy owner.
func (w *LoanWorkflow) OnRequested(lr *models.LoanRequest) error {
	// Ensure owner is loaded.
	var bookCopy models.Copy
	if err := w.db.Preload("Owner").First(&bookCopy, lr.CopyID).Error; err != nil {
		return fmt.Errorf("OnRequested: load copy: %w", err)
	}

	n := models.Notification{
		RecipientID:   bookCopy.OwnerID,
		Type:          "request_received",
		LoanRequestID: &lr.ID,
	}
	if err := w.db.Create(&n).Error; err != nil {
		log.Printf("OnRequested: create notification: %v", err)
	}

	var borrower models.User
	w.db.First(&borrower, lr.BorrowerID)

	subject := "Someone wants to borrow your book"
	html := fmt.Sprintf(
		"<p>Hi %s,</p><p><strong>%s</strong> has requested to borrow your copy of <em>%s</em>.</p>",
		bookCopy.Owner.Name, borrower.Name, bookCopy.Book.Title,
	)
	return w.email.SendEmail(bookCopy.Owner.Email, subject, html)
}

// OnAccepted fires when the owner accepts a loan request.
// In a single transaction it:
//   - Rejects all other pending requests for the same copy.
//   - Updates the copy status to "loaned".
//
// Then it notifies the borrower.
func (w *LoanWorkflow) OnAccepted(lr *models.LoanRequest) error {
	err := w.db.Transaction(func(tx *gorm.DB) error {
		// Reject competing pending requests and notify their borrowers.
		var others []models.LoanRequest
		tx.Where("copy_id = ? AND id != ? AND status = ?", lr.CopyID, lr.ID, "pending").Find(&others)
		for _, other := range others {
			other.Status = "rejected"
			tx.Save(&other)

			n := models.Notification{
				RecipientID:   other.BorrowerID,
				Type:          "request_rejected",
				LoanRequestID: &other.ID,
			}
			tx.Create(&n)
		}

		// Mark copy as loaned.
		if err := tx.Model(&models.Copy{}).Where("id = ?", lr.CopyID).Update("status", "loaned").Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("OnAccepted: transaction: %w", err)
	}

	// Notify the borrower.
	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "request_accepted",
		LoanRequestID: &lr.ID,
	}
	if err := w.db.Create(&n).Error; err != nil {
		log.Printf("OnAccepted: create notification: %v", err)
	}

	// Send email to borrower.
	var borrower models.User
	w.db.First(&borrower, lr.BorrowerID)

	var bookCopy models.Copy
	w.db.Preload("Book").Preload("Owner").First(&bookCopy, lr.CopyID)

	subject := "Your loan request was accepted"
	html := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your request to borrow <em>%s</em> has been accepted by %s. "+
			"Please get in touch to arrange collection.</p>",
		borrower.Name, bookCopy.Book.Title, bookCopy.Owner.Name,
	)
	return w.email.SendEmail(borrower.Email, subject, html)
}

// OnRejected fires when the owner rejects a loan request.
// If no other pending requests exist for the copy, the copy is set back to
// "available".
func (w *LoanWorkflow) OnRejected(lr *models.LoanRequest) error {
	var pendingCount int64
	w.db.Model(&models.LoanRequest{}).
		Where("copy_id = ? AND id != ? AND status = ?", lr.CopyID, lr.ID, "pending").
		Count(&pendingCount)

	if pendingCount == 0 {
		w.db.Model(&models.Copy{}).Where("id = ?", lr.CopyID).Update("status", "available")
	}

	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "request_rejected",
		LoanRequestID: &lr.ID,
	}
	if err := w.db.Create(&n).Error; err != nil {
		log.Printf("OnRejected: create notification: %v", err)
	}

	return nil
}

// OnCancelled fires when the borrower cancels a pending request.
// If no other pending requests exist for the copy, the copy is set back to
// "available".
func (w *LoanWorkflow) OnCancelled(lr *models.LoanRequest) error {
	var pendingCount int64
	w.db.Model(&models.LoanRequest{}).
		Where("copy_id = ? AND id != ? AND status = ?", lr.CopyID, lr.ID, "pending").
		Count(&pendingCount)

	if pendingCount == 0 {
		w.db.Model(&models.Copy{}).Where("id = ?", lr.CopyID).Update("status", "available")
	}

	return nil
}

// OnReturned fires when the owner marks a loan as returned.
// The copy is set back to "available" and the borrower is notified.
func (w *LoanWorkflow) OnReturned(lr *models.LoanRequest) error {
	w.db.Model(&models.Copy{}).Where("id = ?", lr.CopyID).Update("status", "available")

	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "marked_returned",
		LoanRequestID: &lr.ID,
	}
	if err := w.db.Create(&n).Error; err != nil {
		log.Printf("OnReturned: create notification: %v", err)
	}

	var borrower models.User
	w.db.First(&borrower, lr.BorrowerID)

	var bookCopy models.Copy
	w.db.Preload("Book").First(&bookCopy, lr.CopyID)

	subject := "Your loan has been marked as returned"
	html := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your loan of <em>%s</em> has been marked as returned. Thank you!</p>",
		borrower.Name, bookCopy.Book.Title,
	)
	return w.email.SendEmail(borrower.Email, subject, html)
}
