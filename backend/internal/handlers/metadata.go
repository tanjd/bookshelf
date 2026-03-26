package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/repository"
)

// metadataClient is a shared HTTP client with a timeout for all metadata fetches.
var metadataClient = &http.Client{Timeout: 10 * time.Second}

// BookMetadataResult is a normalised search result from any metadata source.
type BookMetadataResult struct {
	Source        string `json:"source"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	CoverURL      string `json:"cover_url"`
	Description   string `json:"description"`
	Publisher     string `json:"publisher"`
	PublishedDate string `json:"published_date"`
	PageCount     int    `json:"page_count"`
	Language      string `json:"language"`
	OLKey         string `json:"ol_key"`
	GoogleBooksID string `json:"google_books_id"`
	BookBrainzID  string `json:"bookbrainz_id,omitempty"`
}

const searchCacheTTL = 1 * time.Hour

// MetadataHandler handles book metadata search routes.
type MetadataHandler struct {
	googleBooksAPIKey string
	jwtSecret         string
	users             repository.UserRepository
	cache             MetadataCache
}

// NewMetadataHandler creates a MetadataHandler.
func NewMetadataHandler(googleBooksAPIKey, jwtSecret string, users repository.UserRepository) *MetadataHandler {
	return &MetadataHandler{
		googleBooksAPIKey: googleBooksAPIKey,
		jwtSecret:         jwtSecret,
		users:             users,
		cache:             NewInMemoryMetadataCache(searchCacheTTL),
	}
}

// RegisterRoutes registers the metadata routes on the given huma API.
func (h *MetadataHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "search-book-metadata",
		Method:      "GET",
		Path:        "/books/metadata/search",
		Tags:        []string{"books"},
		Summary:     "Fan-out metadata search across Open Library and Google Books",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.searchMetadata)

	huma.Register(api, huma.Operation{
		OperationID: "get-ol-description",
		Method:      "GET",
		Path:        "/books/metadata/ol-description",
		Tags:        []string{"books"},
		Summary:     "Fetch work description from Open Library (lazy)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.getOLDescription)
}

type searchMetadataInput struct {
	Q string `query:"q" required:"true" doc:"Search query (title, author, or ISBN)"`
}

type searchMetadataOutput struct {
	Body []BookMetadataResult
}

type olDescriptionInput struct {
	OLKey string `query:"ol_key" required:"true" doc:"Open Library work key e.g. OL12345W"`
}

type olDescriptionOutput struct {
	Body struct {
		Description string `json:"description"`
	}
}

func (h *MetadataHandler) searchMetadata(ctx context.Context, input *searchMetadataInput) (*searchMetadataOutput, error) {
	q := strings.TrimSpace(input.Q)
	if q == "" {
		return &searchMetadataOutput{Body: []BookMetadataResult{}}, nil
	}

	cacheKey := strings.ToLower(q)
	if cached, ok := h.cache.Get(cacheKey); ok {
		log.Info().Str("query", q).Msg("metadata search cache hit")
		return &searchMetadataOutput{Body: cached}, nil
	}

	// Resolve the Google Books API key: prefer the authenticated user's key.
	apiKey := h.googleBooksAPIKey
	if userID, err := middleware.GetRequiredUserID(ctx); err == nil {
		if user, err := h.users.FindByID(userID); err == nil && user.GoogleBooksAPIKey != "" {
			if decrypted, err := decryptField(user.GoogleBooksAPIKey, h.jwtSecret); err == nil {
				apiKey = decrypted
			} else {
				log.Warn().Err(err).Uint("user_id", userID).Msg("could not decrypt user google books api key")
			}
		}
	}

	var mu sync.Mutex
	var results []BookMetadataResult

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := fetchOpenLibrary(q)
		if err != nil {
			log.Warn().Err(err).Msg("open library search failed")
			return
		}
		mu.Lock()
		results = append(results, items...)
		mu.Unlock()
	}()

	if apiKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := fetchGoogleBooks(q, apiKey)
			if err != nil {
				log.Warn().Err(err).Msg("google books search failed")
				return
			}
			mu.Lock()
			results = append(results, items...)
			mu.Unlock()
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := fetchBookBrainz(q)
		if err != nil {
			log.Warn().Err(err).Msg("bookbrainz search failed")
			return
		}
		mu.Lock()
		results = append(results, items...)
		mu.Unlock()
	}()

	wg.Wait()

	consolidated := consolidateResults(results)
	h.cache.Set(cacheKey, consolidated)
	return &searchMetadataOutput{Body: consolidated}, nil
}

func (h *MetadataHandler) getOLDescription(_ context.Context, input *olDescriptionInput) (*olDescriptionOutput, error) {
	workKey := strings.TrimPrefix(input.OLKey, "/works/")
	apiURL := fmt.Sprintf("https://openlibrary.org/works/%s.json", url.PathEscape(workKey))

	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, huma.Error502BadGateway("could not reach Open Library")
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return &olDescriptionOutput{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, huma.Error502BadGateway("could not read Open Library response")
	}

	var work struct {
		Description json.RawMessage `json:"description"`
	}
	if err := json.Unmarshal(body, &work); err != nil {
		return &olDescriptionOutput{}, nil
	}

	var out olDescriptionOutput
	// description may be a plain string or {"type":..., "value": "..."}
	var plain string
	if err := json.Unmarshal(work.Description, &plain); err == nil {
		out.Body.Description = plain
	} else {
		var obj struct {
			Value string `json:"value"`
		}
		if err := json.Unmarshal(work.Description, &obj); err == nil {
			out.Body.Description = obj.Value
		}
	}
	return &out, nil
}

// fetchOpenLibrary calls the OL search API and returns normalised results.
func fetchOpenLibrary(q string) ([]BookMetadataResult, error) {
	log.Info().Str("query", q).Msg("searching Open Library")
	apiURL := fmt.Sprintf(
		"https://openlibrary.org/search.json?q=%s&fields=key,title,author_name,isbn,cover_i&limit=10",
		url.QueryEscape(q),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Docs []struct {
			Key        string   `json:"key"`
			Title      string   `json:"title"`
			AuthorName []string `json:"author_name"`
			ISBN       []string `json:"isbn"`
			CoverI     int64    `json:"cover_i"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	log.Info().Str("query", q).Int("results", len(payload.Docs)).Msg("Open Library search complete")
	results := make([]BookMetadataResult, 0, len(payload.Docs))
	for _, doc := range payload.Docs {
		r := BookMetadataResult{
			Source: "openlibrary",
			Title:  doc.Title,
			OLKey:  doc.Key,
		}
		if len(doc.AuthorName) > 0 {
			r.Author = doc.AuthorName[0]
		}
		if len(doc.ISBN) > 0 {
			r.ISBN = doc.ISBN[0]
		}
		if doc.CoverI > 0 {
			r.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", doc.CoverI)
		}
		results = append(results, r)
	}
	return results, nil
}

// validateGoogleBooksAPIKey makes a minimal test call to verify the key is accepted by Google Books.
func validateGoogleBooksAPIKey(key string) error {
	apiURL := fmt.Sprintf(
		"https://www.googleapis.com/books/v1/volumes?q=test&key=%s&maxResults=1",
		url.QueryEscape(key),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return fmt.Errorf("could not reach Google Books API: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google Books API rejected the key (status %d)", resp.StatusCode)
	}
	return nil
}

// fetchGoogleBooks calls the Google Books API and returns normalised results.
func fetchGoogleBooks(q, apiKey string) ([]BookMetadataResult, error) {
	log.Info().Str("query", q).Msg("searching Google Books")
	apiURL := fmt.Sprintf(
		"https://www.googleapis.com/books/v1/volumes?q=%s&key=%s&maxResults=10",
		url.QueryEscape(q),
		url.QueryEscape(apiKey),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Items []struct {
			ID         string `json:"id"`
			VolumeInfo struct {
				Title               string   `json:"title"`
				Authors             []string `json:"authors"`
				Publisher           string   `json:"publisher"`
				PublishedDate       string   `json:"publishedDate"`
				Description         string   `json:"description"`
				PageCount           int      `json:"pageCount"`
				Language            string   `json:"language"`
				IndustryIdentifiers []struct {
					Type       string `json:"type"`
					Identifier string `json:"identifier"`
				} `json:"industryIdentifiers"`
				ImageLinks struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	log.Info().Str("query", q).Int("results", len(payload.Items)).Msg("Google Books search complete")
	results := make([]BookMetadataResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		vi := item.VolumeInfo
		r := BookMetadataResult{
			Source:        "google_books",
			GoogleBooksID: item.ID,
			Title:         vi.Title,
			Publisher:     vi.Publisher,
			PublishedDate: vi.PublishedDate,
			Description:   vi.Description,
			PageCount:     vi.PageCount,
			Language:      vi.Language,
		}
		if len(vi.Authors) > 0 {
			r.Author = vi.Authors[0]
		}
		// Prefer ISBN-13
		for _, id := range vi.IndustryIdentifiers {
			if id.Type == "ISBN_13" {
				r.ISBN = id.Identifier
				break
			}
		}
		if r.ISBN == "" {
			for _, id := range vi.IndustryIdentifiers {
				if id.Type == "ISBN_10" {
					r.ISBN = id.Identifier
					break
				}
			}
		}
		if thumb := vi.ImageLinks.Thumbnail; thumb != "" {
			r.CoverURL = strings.Replace(thumb, "http://", "https://", 1)
		}
		results = append(results, r)
	}
	return results, nil
}

// fetchBookBrainz calls the BookBrainz search API and returns normalised results.
// BookBrainz does not provide cover images.
func fetchBookBrainz(q string) ([]BookMetadataResult, error) {
	log.Info().Str("query", q).Msg("searching BookBrainz")
	apiURL := fmt.Sprintf(
		"https://api.bookbrainz.org/1/search?q=%s&type=edition&size=10",
		url.QueryEscape(q),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bookbrainz returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Results []struct {
			BBID         string `json:"bbid"`
			DefaultAlias struct {
				Name string `json:"name"`
			} `json:"default-alias"`
			AuthorCredit struct {
				Names []struct {
					Name string `json:"name"`
				} `json:"names"`
			} `json:"author-credit"`
		} `json:"search-results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	log.Info().Str("query", q).Int("results", len(payload.Results)).Msg("BookBrainz search complete")
	results := make([]BookMetadataResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.DefaultAlias.Name == "" {
			continue
		}
		r := BookMetadataResult{
			Source:       "bookbrainz",
			Title:        item.DefaultAlias.Name,
			BookBrainzID: item.BBID,
		}
		if len(item.AuthorCredit.Names) > 0 {
			r.Author = item.AuthorCredit.Names[0].Name
		}
		results = append(results, r)
	}
	return results, nil
}
