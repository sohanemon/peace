package quran

import (
	"context"
	"errors"
)

const maxPages = 1000

// paginate collects every page of a Quran.com endpoint. The callback reports
// the next page from the API pagination block, or nil when the list is done.
func paginate[T any](ctx context.Context, fetch func(context.Context, int) ([]T, *int, error)) ([]T, error) {
	var items []T
	for page := 1; page <= maxPages; page++ {
		pageItems, next, err := fetch(ctx, page)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if next == nil || *next <= page {
			return items, nil
		}
	}
	return nil, errors.New("quran.com API: pagination did not terminate")
}
