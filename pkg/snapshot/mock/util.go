package mock

import (
	"fmt"

	"github.com/golang/mock/gomock"
)

type anyOfMatcher struct {
	values []interface{}
}

func (m anyOfMatcher) Matches(x interface{}) bool {
	for _, v := range m.values {
		if gomock.Eq(v).Matches(x) {
			return true
		}
	}
	return false
}
func (m anyOfMatcher) String() string {
	return fmt.Sprintf("is equal to any of %+v", m.values)
}
func AnyOf(xs ...interface{}) anyOfMatcher {
	return anyOfMatcher{xs}
}
