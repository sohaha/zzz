package watch

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestChangePayloadDoesNotMutateChangedFile(t *testing.T) {
	oldHTTPPath := httpPath
	oldWS := ws
	t.Cleanup(func() {
		httpPath = oldHTTPPath
		ws = oldWS
	})
	httpPath = "/tmp/site"
	ws = nil

	data := &changedFile{
		Name: "src/main.go",
		Path: "/tmp/site/src/main.go",
	}
	msg, ok := changePayload(data)
	if !ok {
		t.Fatal("expected file under the HTTP root to produce a payload")
	}

	if data.Name != "src/main.go" {
		t.Fatalf("changePayload mutated the shared event name: got %q", data.Name)
	}

	var payload changedFile
	if err := json.Unmarshal(msg, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Name != "src/main.go" {
		t.Fatalf("payload name = %q, want src/main.go", payload.Name)
	}
}

func TestChangePayloadRejectsSiblingRoots(t *testing.T) {
	oldHTTPPath := httpPath
	t.Cleanup(func() {
		httpPath = oldHTTPPath
	})
	httpPath = "/tmp/site"

	if _, ok := changePayload(&changedFile{Path: "/tmp/site2/index.html"}); ok {
		t.Fatal("expected /tmp/site2 to be outside /tmp/site")
	}
	if _, ok := changePayload(&changedFile{Path: "/tmp/site"}); ok {
		t.Fatal("expected the root itself not to be a change payload")
	}
}

func TestResolveStaticPathStaysInsideRoot(t *testing.T) {
	oldHTTPPath := httpPath
	t.Cleanup(func() {
		httpPath = oldHTTPPath
	})
	httpPath = filepath.ToSlash(filepath.Join(t.TempDir(), "site"))

	cases := []struct {
		urlPath string
		want    string
	}{
		{"/index.html", httpPath + "/index.html"},
		{"/assets/app.js", httpPath + "/assets/app.js"},
		{"/../outside.txt", httpPath + "/outside.txt"},
		{"/..\\outside.txt", httpPath + "/outside.txt"},
		{"/a/../../outside.txt", httpPath + "/outside.txt"},
	}

	for _, c := range cases {
		got, ok := resolveStaticPath(c.urlPath)
		if !ok {
			t.Fatalf("resolveStaticPath(%q) unexpectedly rejected the path", c.urlPath)
		}
		if got != c.want {
			t.Fatalf("resolveStaticPath(%q) = %q, want %q", c.urlPath, got, c.want)
		}
		if !isWithinDir(got, httpPath) {
			t.Fatalf("resolveStaticPath(%q) escaped %q: %q", c.urlPath, httpPath, got)
		}
	}
}

func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !hasDotDotPrefix(rel))
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}

func TestURLParseAddsDefaultScheme(t *testing.T) {
	host, scheme := urlParse("127.0.0.1:8080")
	if host != "127.0.0.1:8080" || scheme != "http" {
		t.Fatalf("urlParse without scheme = (%q, %q), want (127.0.0.1:8080, http)", host, scheme)
	}

	host, scheme = urlParse("https://example.com")
	if host != "example.com" || scheme != "https" {
		t.Fatalf("urlParse with scheme = (%q, %q), want (example.com, https)", host, scheme)
	}
}
