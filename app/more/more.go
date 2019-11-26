package more

import (
	"errors"
	"github.com/sohaha/zlsgo/zstring"
	"reflect"
)

type Methods struct {
}

func RunMethod(methodName string, vars []string) (err error) {
	methodName = zstring.Ucfirst(methodName)
	reflectValue := reflect.ValueOf(&Methods{})
	dataType := reflectValue.MethodByName(methodName)
	if dataType.Kind() == reflect.Invalid {
		err = errors.New("unknown commands")
		return
	}
	v := make([]reflect.Value, 0)
	v = append(v, reflect.ValueOf(vars))
	dataType.Call(v)
	return
}
