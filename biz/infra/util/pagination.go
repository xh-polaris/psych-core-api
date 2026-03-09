package util

import (
	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func PagedFindOpt(pg *basic.PaginationOptions) *options.FindOptionsBuilder {
	p := pg.GetPage() - 1
	l := pg.GetLimit()
	return options.Find().SetSkip(p * l).SetLimit(l)
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

	return &basic.Pagination{
		Total:   int64(total),
		Page:    pg.GetPage(),
		Limit:   pg.GetLimit(),
		HasNext: pg.GetPage()*pg.GetLimit() < int64(total),
	}
}
