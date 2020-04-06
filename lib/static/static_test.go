package static_test

import (
	"testing"

	"github.com/sohaha/zlsgo"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zzz/lib/static"
)

func TestStatic(t *testing.T) {
	tt := zlsgo.NewTest(t)

	tt.EqualExit(true, zfile.FileExist("../../LICENSE"))

	g, _ := static.Group("../../")
	// g.SetCaller(2)
	s, err := g.MustString("LICENSE")
	tt.EqualNil(err)
	tt.Equal(11358, len(s))

}
