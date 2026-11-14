package static

import (
	"os"
	"testing"

	"github.com/sohaha/zlsgo"
	"github.com/sohaha/zlsgo/zfile"
)

func TestStatic(t *testing.T) {
	tt := zlsgo.NewTest(t)

	wd, _ := os.Getwd()
	tt.EqualExit(true, zfile.FileExist(wd+"/../../LICENSE"))

	g, _ := Group(wd + "/../..")
	// g.SetCaller(2)
	s, err := g.MustString("LICENSE")
	tt.EqualNil(err)
	tt.Equal(11358, len(s))

}
