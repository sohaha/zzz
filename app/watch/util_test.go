package watch

import "testing"

func TestIsExcept(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		path     string
		want     bool
	}{
		{"通配全部", []string{"*"}, "/p/any.txt", true},
		{"空规则", nil, "/p/any.txt", false},
		{"目录名命中", []string{"node_modules"}, "/p/node_modules", true},
		{"目录名未命中", []string{"node_modules"}, "/p/src", false},
		{"星号前缀目录", []string{"*/dist"}, "/p/dist/bundle.js", true},
		{"递归目录", []string{"**/dist/**"}, "/p/src/dist/a.js", true},
		{"递归目录未命中", []string{"**/dist/**"}, "/p/src/docs/a.js", false},
		{"后缀目录", []string{"dist/**"}, "./dist/a.go", true},
		{"基名通配", []string{"*.min.js"}, "/p/assets/b.min.js", true},
		{"基名通配未命中", []string{"*.min.js"}, "/p/assets/b.js", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isExcept(c.patterns, c.path); got != c.want {
				t.Fatalf("isExcept(%v, %q) = %v, want %v", c.patterns, c.path, got, c.want)
			}
			// Call twice to exercise the compiled-pattern cache path.
			if got := isExcept(c.patterns, c.path); got != c.want {
				t.Fatalf("isExcept(%v, %q) second call = %v, want %v", c.patterns, c.path, got, c.want)
			}
		})
	}
}
