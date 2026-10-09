package service

import (
	"github.com/Anshul563/edvance-project/services/social-service/internal/config"
)

type Pagination struct {
	Page  int
	Limit int
	Skip  int
}

func NewPagination(page, limit int, cfg config.PaginationConfig) Pagination {
	if page <= 0 {
		page = 1
	}

	maxLimit := cfg.MaxPageSize
	if maxLimit <= 0 {
		maxLimit = 100
	}

	defaultLimit := cfg.DefaultPageSize
	if defaultLimit <= 0 {
		defaultLimit = 20
	}

	if limit <= 0 {
		limit = defaultLimit
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	return Pagination{
		Page:  page,
		Limit: limit,
		Skip:  (page - 1) * limit,
	}
}
