package app

import (
	"net/http"
	"strconv"
)

// maxListPageSize caps every paginated admin list, matching the 100-row cap
// used by the log endpoints.
const maxListPageSize = 100

// listPage reads the page/page_size query params shared by all paginated admin
// lists: page starts at 1, page_size defaults to 50 and is capped at maxListPageSize.
func listPage(r *http.Request) (page, pageSize, offset int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > maxListPageSize {
		pageSize = 50
	}
	offset = (page - 1) * pageSize
	return page, pageSize, offset
}

type pagedList struct {
	Data     any `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func writePaged(w http.ResponseWriter, data any, total, page, pageSize int) {
	writeJSON(w, 200, pagedList{Data: data, Total: total, Page: page, PageSize: pageSize})
}