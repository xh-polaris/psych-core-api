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
