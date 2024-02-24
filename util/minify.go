package util

import (
	"strconv"

	"github.com/sohaha/zlsgo/zjson"
	"github.com/sohaha/zlsgo/znet"
	"github.com/tdewolff/minify/v2"

	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

func MinifyHandle(c *znet.Context) {
	m := minify.New()
	m.AddFunc("2", css.Minify)
	m.AddFunc("0", html.Minify)
	m.AddFunc("1", js.Minify)

	// m.AddFunc("s", svg.Minify)
	// m.AddFuncRegexp(regexp.MustCompile("[/+]j$"), json.Minify)
	// m.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)

	j, err := c.GetJSONs()
	if err != nil {
		c.ApiJSON(500, err.Error(), "")
		return
	}

	i := 0
	var codes []string
	j.ForEach(func(key, value *zjson.Res) bool {
		code, err := m.String(strconv.Itoa(i), value.String())
		i++
		if err != nil {
			code = value.String()
		}
		codes = append(codes, code)
		return true
	})

	c.ApiJSON(200, "", codes)
}
