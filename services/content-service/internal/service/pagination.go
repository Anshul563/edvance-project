package service

// Page is the paginated envelope every list endpoint returns. Items are
// service entities; handlers map them to response DTOs.
type Page[T any] struct {
	Items   []T   `json:"items"`
	Page    int   `json:"page"`
	Limit   int   `json:"limit"`
	Total   int64 `json:"total"`
	HasNext bool  `json:"hasNext"`
}

// Limits carries the configurable bounds loaded from environment
// variables. Nothing in the service hardcodes platform policy that an
// operator may legitimately tune.
type Limits struct {
	DefaultPage            int
	MaxPage                int
	MaxShortDurationSeconds int
	MaxTitleLength         int
	MaxDescriptionLength   int
	MaxPostContentLength   int
	MaxTagsPerItem         int
}

// NormalizePagination clamps request parameters into a safe window:
//
//	page < 1        -> 1
//	limit <= 0      -> default page size
//	limit > max     -> max page size
//
// Clients can never make the service scan an unbounded result set.
func NormalizePagination(page int, limit int, limits Limits) (int, int) {
	if page < 1 {
		page = 1
	}

	if limit <= 0 {
		limit = limits.DefaultPage
	}

	if limit > limits.MaxPage {
		limit = limits.MaxPage
	}

	return page, limit
}

// offset converts a 1-based page into a SQL offset.
func offset(page int, limit int) int {
	return (page - 1) * limit
}

// newPage assembles the envelope, computing hasNext from the total.
func newPage[T any](items []T, page int, limit int, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}

	return Page[T]{
		Items:   items,
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: int64(page*limit) < total,
	}
}
