package util

import (
	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func PagedFindOpt(pg *basic.PaginationOptions) *options.FindOptionsBuilder {
	page := pg.GetPage()
	if page < 1 {
		page = 1
	}
	limit := pg.GetLimit()
	if limit < 1 {
		limit = 10
	}

	return options.Find().SetSkip((page - 1) * limit).SetLimit(limit)
}

func PagedIndex(total int32, pg *basic.PaginationOptions) (int, int) {
	page, size := int(pg.GetPage()), int(pg.GetLimit())
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	startIdx := (page - 1) * size
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + size
	if startIdx >= int(total) {
		startIdx = 0
	}
	if endIdx > int(total) {
		endIdx = int(total)
	}

	return startIdx, endIdx
}

func PaginationRes(total int32, pg *basic.PaginationOptions) *basic.Pagination {
	if total == 0 {
		return &basic.Pagination{Total: 0, HasNext: false}
	}

	page := pg.GetPage()
	if page < 1 {
		page = 1
	}
	limit := pg.GetLimit()
	if limit < 1 {
		limit = 10
	}

	return &basic.Pagination{
		Total:   int64(total),
		Page:    page,
		Limit:   limit,
		HasNext: page*limit < int64(total),
	}
}
