package app

import (
	"math"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestListPageDefaultsAndCap(t *testing.T) {
	for _, test := range []struct {
		name     string
		query    string
		page     int
		pageSize int
		offset   int
	}{
		{name: "defaults", query: "", page: 1, pageSize: 50, offset: 0},
		{name: "normalizes invalid values", query: "page=0&page_size=0", page: 1, pageSize: 50, offset: 0},
		{name: "caps page size", query: "page=2&page_size=101", page: 2, pageSize: 50, offset: 50},
		{name: "accepts maximum page size", query: "page=3&page_size=100", page: 3, pageSize: 100, offset: 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, pageSize, offset := listPage(httptest.NewRequest("GET", "/admin/items?"+test.query, nil))
			if page != test.page || pageSize != test.pageSize || offset != test.offset {
				t.Fatalf("listPage() = (%d, %d, %d), want (%d, %d, %d)", page, pageSize, offset, test.page, test.pageSize, test.offset)
			}
		})
	}
}

func TestListPageAvoidsOffsetOverflow(t *testing.T) {
	page, pageSize, offset := listPage(httptest.NewRequest("GET", "/admin/items?page="+strconv.Itoa(math.MaxInt)+"&page_size=100", nil))
	if page != math.MaxInt || pageSize != 100 || offset != math.MaxInt {
		t.Fatalf("listPage() = (%d, %d, %d), want (%d, %d, %d)", page, pageSize, offset, math.MaxInt, 100, math.MaxInt)
	}
}
