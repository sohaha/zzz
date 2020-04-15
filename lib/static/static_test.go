package static

import (
	"testing"

	"github.com/sohaha/zlsgo"
	"github.com/sohaha/zlsgo/zfile"
)

func TestStatic(t *testing.T) {
	tt := zlsgo.NewTest(t)

	tt.EqualExit(true, zfile.FileExist("../../LICENSE"))

	g, _ := Group("../../")
	// g.SetCaller(2)
	s, err := g.MustString("LICENSE")
	tt.EqualNil(err)
	tt.Equal(11358, len(s))

}
