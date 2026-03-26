package handlers

import (
	"regexp"
	"sort"
	"strings"
)

// nonAlphanumSpace matches any character that is not a lowercase letter, digit, or space.
var nonAlphanumSpace = regexp.MustCompile(`[^a-z0-9 ]+`)

// sourcePriority returns a numeric priority for a source (lower = higher priority).
func sourcePriority(source string) int {
	switch source {
	case "google_books":
		return 0
	case "openlibrary":
		return 1
	default:
		return 2
	}
}

// normalizeISBN strips hyphens/spaces and converts ISBN-10 to ISBN-13.
// Returns "" if the input doesn't produce a valid 10- or 13-digit ISBN.
func normalizeISBN(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ToUpper(s)

	switch len(s) {
	case 13:
		// Verify all digits
		for _, c := range s {
			if c < '0' || c > '9' {
				return ""
			}
		}
		return s
	case 10:
		// ISBN-10: last char may be X (= 10)
		for i, c := range s {
			if i == 9 {
				if c != 'X' && (c < '0' || c > '9') {
					return ""
				}
			} else if c < '0' || c > '9' {
				return ""
			}
		}
		// Convert to ISBN-13: prepend "978", drop old check digit, compute new one
		prefix := "978" + s[:9]
		check := isbn13CheckDigit(prefix)
		return prefix + string([]byte{'0' + check})
	default:
		return ""
	}
}

// isbn13CheckDigit computes the ISBN-13 check digit for a 12-digit string.
func isbn13CheckDigit(digits string) byte {
	sum := 0
	for i, c := range digits {
		d := int(c - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	return byte((10 - (sum % 10)) % 10)
}

// normalizeTitleAuthor returns a deduplication key from title and author.
func normalizeTitleAuthor(title, author string) string {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = nonAlphanumSpace.ReplaceAllString(s, "")
		s = strings.Join(strings.Fields(s), " ")
		return s
	}
	return norm(title) + "|" + norm(author)
}

// deduplicateIntoGroups groups results that refer to the same book.
// ISBN (normalized to ISBN-13) is the primary key; title+author is the fallback.
func deduplicateIntoGroups(results []BookMetadataResult) [][]BookMetadataResult {
	groups := [][]BookMetadataResult{}
	isbnIndex := map[string]int{}
	titleAuthorIndex := map[string]int{}

	for _, r := range results {
		normISBN := normalizeISBN(r.ISBN)
		normTA := normalizeTitleAuthor(r.Title, r.Author)

		idx, found := -1, false

		if normISBN != "" {
			if i, ok := isbnIndex[normISBN]; ok {
				idx, found = i, true
			}
		}
		if !found && normTA != "|" {
			if i, ok := titleAuthorIndex[normTA]; ok {
				idx, found = i, true
			}
		}

		if found {
			groups[idx] = append(groups[idx], r)
			// Register ISBN in the index if not already there
			if normISBN != "" {
				if _, ok := isbnIndex[normISBN]; !ok {
					isbnIndex[normISBN] = idx
				}
			}
		} else {
			idx = len(groups)
			groups = append(groups, []BookMetadataResult{r})
			if normISBN != "" {
				isbnIndex[normISBN] = idx
			}
			if normTA != "|" {
				titleAuthorIndex[normTA] = idx
			}
		}
	}

	return groups
}

// mergeGroup merges a group of results (same book, multiple sources) into one.
// Google Books fields take priority, then Open Library, then BookBrainz.
func mergeGroup(group []BookMetadataResult) BookMetadataResult {
	// Sort by source priority so we pick fields from the best source first
	sorted := make([]BookMetadataResult, len(group))
	copy(sorted, group)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sourcePriority(sorted[i].Source) < sourcePriority(sorted[j].Source)
	})

	merged := BookMetadataResult{}

	// Determine the richest source that contributed
	merged.Source = sorted[0].Source

	// Pick first non-empty value in priority order for shared fields
	for _, r := range sorted {
		if merged.Title == "" {
			merged.Title = r.Title
		}
		if merged.Author == "" {
			merged.Author = r.Author
		}
		if merged.ISBN == "" {
			merged.ISBN = r.ISBN
		}
		if merged.CoverURL == "" {
			merged.CoverURL = r.CoverURL
		}
		if merged.Description == "" {
			merged.Description = r.Description
		}
		if merged.Publisher == "" {
			merged.Publisher = r.Publisher
		}
		if merged.PublishedDate == "" {
			merged.PublishedDate = r.PublishedDate
		}
		if merged.PageCount == 0 {
			merged.PageCount = r.PageCount
		}
		if merged.Language == "" {
			merged.Language = r.Language
		}
		// Accumulate all source IDs
		if merged.OLKey == "" {
			merged.OLKey = r.OLKey
		}
		if merged.GoogleBooksID == "" {
			merged.GoogleBooksID = r.GoogleBooksID
		}
		if merged.BookBrainzID == "" {
			merged.BookBrainzID = r.BookBrainzID
		}
	}

	return merged
}

// scoreResult returns a completeness score for ranking.
func scoreResult(r BookMetadataResult) int {
	score := 0
	if r.CoverURL != "" {
		score += 2
	}
	if r.Description != "" {
		score += 2
	}
	if r.ISBN != "" {
		score += 1
	}
	if r.Publisher != "" {
		score += 1
	}
	if r.PageCount > 0 {
		score += 1
	}
	// Bonus for multi-source confidence
	sources := 0
	if r.OLKey != "" {
		sources++
	}
	if r.GoogleBooksID != "" {
		sources++
	}
	if r.BookBrainzID != "" {
		sources++
	}
	if sources > 1 {
		score += sources - 1
	}
	return score
}

// consolidateResults deduplicates, merges, and ranks results from all sources.
func consolidateResults(results []BookMetadataResult) []BookMetadataResult {
	if len(results) == 0 {
		return []BookMetadataResult{}
	}

	groups := deduplicateIntoGroups(results)

	merged := make([]BookMetadataResult, 0, len(groups))
	for _, group := range groups {
		merged = append(merged, mergeGroup(group))
	}

	sort.SliceStable(merged, func(i, j int) bool {
		si, sj := scoreResult(merged[i]), scoreResult(merged[j])
		if si != sj {
			return si > sj
		}
		return merged[i].Title < merged[j].Title
	})

	return merged
}
