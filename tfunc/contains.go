package tfunc

import (
	"reflect"
	"strings"
)

// contains is a function that have reverse arguments of "in" and is designed to
// be used as a pipe instead of a function:
//
// 		{{ l | contains "thing" }}
//
func contains(v, l interface{}) (bool, error) {
	return in(l, v)
}

// containsSomeFunc returns functions to implement each of the following:
//
// 1. containsAll    - true if (∀x ∈ v then x ∈ l); false otherwise
// 2. containsAny    - true if (∃x ∈ v such that x ∈ l); false otherwise
// 3. containsNone   - true if (∀x ∈ v then x ∉ l); false otherwise
// 2. containsNotAll - true if (∃x ∈ v such that x ∉ l); false otherwise
//
// ret_true - return true at end of loop for none/all; false for any/notall
// invert   - invert block test for all/notall
func containsSomeFunc(retTrue, invert bool) func([]interface{}, interface{}) (bool, error) {
	return func(v []interface{}, l interface{}) (bool, error) {
		for i := 0; i < len(v); i++ {
			if ok, _ := in(l, v[i]); ok != invert {
				return !retTrue, nil
			}
		}
		return retTrue, nil
	}
}

// in searches for a given value in a given interface.
func in(l, v interface{}) (bool, error) {
	lv := reflect.ValueOf(l)
	vv := reflect.ValueOf(v)

	switch lv.Kind() {
	case reflect.Array, reflect.Slice:
		// if the slice contains 'interface' elements, then the element needs to be extracted directly to examine its type,
		// otherwise it will just resolve to 'interface'.
		var interfaceSlice []interface{}
		if reflect.TypeOf(l).Elem().Kind() == reflect.Interface {
			interfaceSlice = l.([]interface{})
		}

		for i := 0; i < lv.Len(); i++ {
			var lvv reflect.Value
			if interfaceSlice != nil {
				lvv = reflect.ValueOf(interfaceSlice[i])
			} else {
				lvv = lv.Index(i)
			}

			switch lvv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				switch vv.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if vv.Int() == lvv.Int() {
						return true, nil
					}
				}
			case reflect.Float32, reflect.Float64:
				switch vv.Kind() {
				case reflect.Float32, reflect.Float64:
					if vv.Float() == lvv.Float() {
						return true, nil
					}
				}
			case reflect.String:
				if vv.Type() == lvv.Type() && vv.String() == lvv.String() {
					return true, nil
				}
			}
		}
	case reflect.String:
		if vv.Type() == lv.Type() && strings.Contains(lv.String(), vv.String()) {
			return true, nil
		}
	}

	return false, nil
}
