package state

import (
	"reflect"
)

func index(slice interface{}, dxFn func(int) string) map[string]int {
	idx := map[string]int{}

	if reflect.TypeOf(slice).Kind() != reflect.Slice {
		panic("first argument passed to newIndex must be a slice")
	}

	s := reflect.ValueOf(slice)
	for i := 0; i < s.Len(); i++ {
		idx[dxFn(i)] = i
	}

	return idx
}

func int64p(i int) *int64 {
	s := int64(i)
	return &s
}
