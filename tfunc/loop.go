package tfunc

import (
	"fmt"
	"reflect"
)

// loop accepts varying parameters and differs its behavior. If given one
// parameter, loop will return a goroutine that begins at 0 and loops until the
// given int, increasing the index by 1 each iteration. If given two parameters,
// loop will return a goroutine that begins at the first parameter and loops
// up to but not including the second parameter.
//
//    // Prints 0 1 2 3 4
// 		for _, i := range loop(5) {
// 			print(i)
// 		}
//
//    // Prints 5 6 7
// 		for _, i := range loop(5, 8) {
// 			print(i)
// 		}
//
func loop(ifaces ...interface{}) (<-chan int64, error) {

	to64 := func(i interface{}) (int64, error) {
		v := reflect.ValueOf(i)
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
			reflect.Int64:
			return int64(v.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
			reflect.Uint64:
			return int64(v.Uint()), nil
		case reflect.String:
			return parseInt(v.String())
		}
		return 0, fmt.Errorf("loop: bad argument type: %T", i)
	}

	var i1, i2 interface{}
	switch len(ifaces) {
	case 1:
		i1, i2 = 0, ifaces[0]
	case 2:
		i1, i2 = ifaces[0], ifaces[1]
	default:
		return nil, fmt.Errorf("loop: wrong number of arguments, expected "+
			"1 or 2, but got %d", len(ifaces))
	}

	start, err := to64(i1)
	if err != nil {
		return nil, err
	}
	stop, err := to64(i2)
	if err != nil {
		return nil, err
	}

	ch := make(chan int64)

	go func() {
		for i := start; i < stop; i++ {
			ch <- i
		}
		close(ch)
	}()

	return ch, nil
}
