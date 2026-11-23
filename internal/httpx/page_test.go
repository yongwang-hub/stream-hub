package httpx

import "testing"

func TestPageInputNormalizeAndOffset(t *testing.T) {
	for _, tc := range []struct {
		in                  PageInput
		page, pageSize, off int
	}{
		{PageInput{}, 1, DefaultPageSize, 0},
		{PageInput{Page: 3, PageSize: 10}, 3, 10, 20},
		{PageInput{Page: -1, PageSize: 1000}, 1, MaxPageSize, 0},
	} {
		got := tc.in.Normalize()
		if got.Page != tc.page || got.PageSize != tc.pageSize || got.Offset() != tc.off {
			t.Errorf("%+v: got page=%d size=%d offset=%d, want %d/%d/%d",
				tc.in, got.Page, got.PageSize, got.Offset(), tc.page, tc.pageSize, tc.off)
		}
	}
}
