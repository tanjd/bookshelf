// Package models defines the GORM database models for the bookshelf app.
package models

import "time"

// User represents a registered member of the church community.
type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Phone     string    `json:"phone"`
	Password  string    `gorm:"not null" json:"-"`
	Verified  bool      `gorm:"default:false" json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

// Book is a title in the library catalogue.
type Book struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	Title       string `gorm:"not null" json:"title"`
	Author      string `gorm:"not null" json:"author"`
	ISBN        string `json:"isbn"`
	OLKey       string `gorm:"uniqueIndex" json:"ol_key"`
	CoverURL    string `json:"cover_url"`
	Description string `json:"description"`
	Copies      []Copy `json:"copies,omitempty"`
}

// Copy is a physical instance of a Book owned by a church member.
// Status values: available | requested | loaned | unavailable
type Copy struct {
	ID        uint   `gorm:"primarykey" json:"id"`
	BookID    uint   `gorm:"not null" json:"book_id"`
	OwnerID   uint   `gorm:"not null" json:"owner_id"`
	Condition string `json:"condition"` // good | fair | worn
	Notes     string `json:"notes"`
	Status    string `gorm:"default:'available'" json:"status"`
	Book      Book   `json:"book,omitempty"`
	Owner     User   `json:"owner,omitempty"`
}

// LoanRequest tracks a borrower's request to borrow a specific Copy.
// Status values: pending | accepted | rejected | cancelled | returned
type LoanRequest struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	CopyID      uint       `gorm:"not null" json:"copy_id"`
	BorrowerID  uint       `gorm:"not null" json:"borrower_id"`
	Message     string     `json:"message"`
	Status      string     `gorm:"default:'pending'" json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	RespondedAt *time.Time `json:"responded_at"`
	LoanedAt    *time.Time `json:"loaned_at"`
	ReturnedAt  *time.Time `json:"returned_at"`
	Copy        Copy       `json:"copy,omitempty"`
	Borrower    User       `json:"borrower,omitempty"`
}

// Notification is an in-app alert delivered to a user.
// Type values: request_received | request_accepted | request_rejected |
//
//	marked_loaned | marked_returned
type Notification struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	RecipientID   uint      `gorm:"not null" json:"recipient_id"`
	Type          string    `json:"type"`
	LoanRequestID *uint     `json:"loan_request_id"`
	Read          bool      `gorm:"default:false" json:"read"`
	CreatedAt     time.Time `json:"created_at"`
}
