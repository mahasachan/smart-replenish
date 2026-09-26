package httpapi

import (
	"net/http/httptest"
	"smartreplenish/internal/domain"
	"strings"
	"testing"
)

func TestDecodeRejectsNonObjectsAndTrailingData(t *testing.T) {
	for _, body := range []string{"null", "[]", `{} {}`, `{"unknown":true}`, strings.Repeat(" ", 1<<20) + `{}`} {
		r := httptest.NewRequest("PUT", "/", strings.NewReader(body))
		var state domain.Membership
		if err := decode(httptest.NewRecorder(), r, &state); err == nil {
			t.Errorf("accepted malformed body beginning %.30s", body)
		}
	}
	r := httptest.NewRequest("PUT", "/", strings.NewReader(`{"in_cart":true,"in_list":false}`))
	var state domain.Membership
	if err := decode(httptest.NewRecorder(), r, &state); err != nil || !state.InCart {
		t.Fatal(err)
	}
}
