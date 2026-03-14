package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
)

// BookHandler holds dependencies for book routes.
type BookHandler struct {
	db *gorm.DB
}

// NewBookHandler creates a new BookHandler.
func NewBookHandler(db *gorm.DB) *BookHandler {
	return &BookHandler{db: db}
}

// bookResponse wraps a Book and adds the computed available_copies count.
type bookResponse struct {
	models.Book
	AvailableCopies int64 `json:"available_copies"`
}

// --- Input / Output types ---

type listBooksInput struct {
	Q     string `query:"q" doc:"Search by title or author"`
	OLKey string `query:"ol_key" doc:"Filter by exact Open Library key (returns single book)"`
}

type listBooksOutput struct{ Body []bookResponse }

type getBookInput struct {
	ID uint `path:"id" doc:"Book ID"`
}

type getBookOutput struct{ Body bookResponse }

type createBookInput struct {
	Body struct {
		Title       string `json:"title" required:"true" minLength:"1" doc:"Book title"`
		Author      string `json:"author" required:"true" minLength:"1" doc:"Author name"`
		ISBN        string `json:"isbn,omitempty" doc:"ISBN-13"`
		OLKey       string `json:"ol_key,omitempty" doc:"Open Library key for deduplication"`
		CoverURL    string `json:"cover_url,omitempty" doc:"Cover image URL"`
		Description string `json:"description,omitempty" doc:"Book description"`
	}
}

type createBookOutput struct{ Body models.Book }

// --- Route registration ---

// RegisterRoutes registers all book routes on the given huma API.
func (h *BookHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-books",
		Method:      "GET",
		Path:        "/books",
		Tags:        []string{"books"},
		Summary:     "List books (optionally search or filter by Open Library key)",
	}, h.listBooks)

	huma.Register(api, huma.Operation{
		OperationID: "get-book",
		Method:      "GET",
		Path:        "/books/{id}",
		Tags:        []string{"books"},
		Summary:     "Get a book by ID",
	}, h.getBook)

	huma.Register(api, huma.Operation{
		OperationID:   "create-book",
		Method:        "POST",
		Path:          "/books",
		Tags:          []string{"books"},
		Summary:       "Create or upsert a book by Open Library key",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.createBook)
}

// --- Handlers ---

func (h *BookHandler) listBooks(_ context.Context, input *listBooksInput) (*listBooksOutput, error) {
	if input.OLKey != "" {
		var book models.Book
		if err := h.db.Where("ol_key = ?", input.OLKey).Preload("Copies").First(&book).Error; err != nil {
			return nil, huma.Error404NotFound("book not found")
		}
		return &listBooksOutput{Body: []bookResponse{h.toBookResponse(book)}}, nil
	}

	var books []models.Book
	tx := h.db.Preload("Copies")
	if input.Q != "" {
		like := "%" + input.Q + "%"
		tx = tx.Where("title LIKE ? OR author LIKE ?", like, like)
	}
	if err := tx.Find(&books).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not fetch books")
	}

	resp := make([]bookResponse, len(books))
	for i, b := range books {
		resp[i] = h.toBookResponse(b)
	}
	return &listBooksOutput{Body: resp}, nil
}

func (h *BookHandler) getBook(_ context.Context, input *getBookInput) (*getBookOutput, error) {
	var book models.Book
	if err := h.db.Preload("Copies.Owner").First(&book, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("book not found")
	}

	// Redact sensitive owner fields — expose name only.
	for i := range book.Copies {
		book.Copies[i].Owner = models.User{
			ID:   book.Copies[i].Owner.ID,
			Name: book.Copies[i].Owner.Name,
		}
	}

	return &getBookOutput{Body: h.toBookResponse(book)}, nil
}

func (h *BookHandler) createBook(ctx context.Context, input *createBookInput) (*createBookOutput, error) {
	if _, err := middleware.GetRequiredUserID(ctx); err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	// Upsert by ol_key: return existing record when the key is already known.
	if input.Body.OLKey != "" {
		var existing models.Book
		if err := h.db.Where("ol_key = ?", input.Body.OLKey).First(&existing).Error; err == nil {
			return &createBookOutput{Body: existing}, nil
		}
	}

	book := models.Book{
		Title:       input.Body.Title,
		Author:      input.Body.Author,
		ISBN:        input.Body.ISBN,
		OLKey:       input.Body.OLKey,
		CoverURL:    input.Body.CoverURL,
		Description: input.Body.Description,
	}
	if err := h.db.Create(&book).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not create book")
	}

	return &createBookOutput{Body: book}, nil
}

// toBookResponse computes the available_copies count and wraps the book.
func (h *BookHandler) toBookResponse(book models.Book) bookResponse {
	var count int64
	h.db.Model(&models.Copy{}).Where("book_id = ? AND status = ?", book.ID, "available").Count(&count)
	return bookResponse{Book: book, AvailableCopies: count}
}
