package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiroki1117/gyazo-cli/internal/config"
	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
)

const testID = "8980c52421e452ac3355ca3e5cfe7a0c"

// result is the outcome of a command run.
type result struct {
	out, errOut string
	err         error
}

// runCmd runs a command against a test API server with env and stdin.
// The test server serves both the API and the Upload API.
func runCmd(t *testing.T, handler http.Handler, env map[string]string, stdin string, args ...string) result {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// Isolate the config file in a temp dir so the real one is never read.
	merged := map[string]string{
		config.EnvBaseURL:     srv.URL,
		config.EnvUploadURL:   srv.URL,
		config.EnvConfigFile:  filepath.Join(t.TempDir(), "config.json"),
		config.EnvMaxRetries:  "0",
		config.EnvAccessToken: "testtoken",
	}
	maps.Copy(merged, env)
	var out, errOut bytes.Buffer
	app := &App{
		IO:     IOStreams{In: strings.NewReader(stdin), Out: &out, ErrOut: &errOut},
		Getenv: func(k string) string { return merged[k] },
	}
	cmd := NewRootCmd(app)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return result{out.String(), errOut.String(), err}
}

// imageAPI serves a fixed image at /api/images/<testID> and its file at /files/<testID>.png.
func imageAPI() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/images/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != testID {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"image_id":%q,"permalink_url":"https://gyazo.com/%[1]s","url":"http://%s/files/%[1]s.png","type":"png",
			"created_at":"2018-07-24T07:33:24.771Z","metadata":{"app":"Chrome","title":"Page","url":"https://example.com","desc":""},
			"ocr":{"locale":"en","description":"hello world"}}`, testID, r.Host)
	})
	mux.HandleFunc("GET /files/{name}", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "PNGDATA")
	})
	mux.HandleFunc("DELETE /api/images/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"image_id":%q,"type":"png"}`, r.PathValue("id"))
	})
	return mux
}

func TestImageGetAndOCR(t *testing.T) {
	for _, arg := range []string{testID, "https://gyazo.com/" + testID, "https://i.gyazo.com/" + testID + ".png"} {
		r := runCmd(t, imageAPI(), nil, "", "image", "ocr", arg)
		if r.err != nil || r.out != "hello world\n" {
			t.Errorf("ocr %s: out = %q, err = %v", arg, r.out, r.err)
		}
	}
	r := runCmd(t, imageAPI(), nil, "", "image", "get", testID)
	if r.err != nil || !strings.Contains(r.out, "Title:") || !strings.Contains(r.out, "https://gyazo.com/"+testID) {
		t.Errorf("get: out = %q, err = %v", r.out, r.err)
	}
}

func TestImageList(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Total-Count", "41")
		w.Header().Set("X-Current-Page", "1")
		w.Header().Set("X-Per-Page", "20")
		fmt.Fprint(w, `[{"image_id":"a","permalink_url":"https://gyazo.com/a","type":"png","metadata":{"title":"T"}}]`)
	})
	r := runCmd(t, api, nil, "", "image", "list")
	if r.err != nil || !strings.Contains(r.out, "https://gyazo.com/a") || !strings.Contains(r.errOut, "page 1/3 (41 images)") {
		t.Errorf("out = %q, errOut = %q, err = %v", r.out, r.errOut, r.err)
	}
	r = runCmd(t, api, nil, "", "image", "list", "--as", "scrapbox")
	if r.err != nil || r.out != "[https://gyazo.com/a]\n" {
		t.Errorf("--as scrapbox: out = %q, err = %v", r.out, r.err)
	}
	r = runCmd(t, api, nil, "", "image", "list", "-o", "json")
	var list gyazo.ImageList
	if err := json.Unmarshal([]byte(r.out), &list); err != nil || list.TotalCount != 41 || list.Images[0].ImageID != "a" {
		t.Errorf("json: %q, %v", r.out, err)
	}
}

func TestUpload(t *testing.T) {
	var gotApp, gotPolicy, gotName string
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		gotApp, gotPolicy = r.FormValue("app"), r.FormValue("access_policy")
		_, h, err := r.FormFile("imagedata")
		if err != nil {
			t.Fatal(err)
		}
		gotName = h.Filename
		fmt.Fprint(w, `{"image_id":"new","permalink_url":"https://gyazo.com/new","url":"https://i.gyazo.com/new.png","type":"png"}`)
	})

	file := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(file, []byte("\x89PNG\r\n\x1a\nxxxx"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := runCmd(t, api, nil, "", "upload", file, "--as", "markdown")
	if r.err != nil || r.out != "![](https://i.gyazo.com/new.png)\n" || gotName != "shot.png" {
		t.Errorf("out = %q, name = %q, err = %v", r.out, gotName, r.err)
	}

	// Defaults come from the config file; flags override them.
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.WriteFile(cfgPath, config.File{Upload: config.UploadDefaults{App: "gy", AccessPolicy: "only_me"}}); err != nil {
		t.Fatal(err)
	}
	r = runCmd(t, api, map[string]string{config.EnvConfigFile: cfgPath}, "PNGDATA", "upload", "-", "--access-policy", "anyone", "-o", "json")
	if r.err != nil || gotApp != "gy" || gotPolicy != "anyone" || gotName != "stdin" {
		t.Errorf("app = %q, policy = %q, name = %q, err = %v", gotApp, gotPolicy, gotName, r.err)
	}
	var images []gyazo.Image
	if err := json.Unmarshal([]byte(r.out), &images); err != nil || len(images) != 1 {
		t.Errorf("json: %q, %v", r.out, err)
	}
}

func TestImageDownload(t *testing.T) {
	dir := t.TempDir()
	r := runCmd(t, imageAPI(), nil, "", "image", "download", testID, "--dir", dir)
	want := filepath.Join(dir, testID+".png")
	if r.err != nil || r.out != want+"\n" {
		t.Fatalf("out = %q, err = %v", r.out, r.err)
	}
	if b, _ := os.ReadFile(want); string(b) != "PNGDATA" {
		t.Errorf("file content = %q", b)
	}
	if r := runCmd(t, imageAPI(), nil, "", "image", "download", testID, "--dir", dir); r.err == nil || !strings.Contains(r.err.Error(), "already exists") {
		t.Errorf("expected exists error, got %v", r.err)
	}
	if r := runCmd(t, imageAPI(), nil, "", "image", "download", testID, "--stdout"); r.err != nil || r.out != "PNGDATA" {
		t.Errorf("--stdout: out = %q, err = %v", r.out, r.err)
	}
}

func TestImageDelete(t *testing.T) {
	r := runCmd(t, imageAPI(), nil, "", "image", "delete", testID)
	if r.err != nil || !strings.Contains(r.errOut, "--yes") || strings.Contains(r.out, "Deleted") {
		t.Errorf("preview: out = %q, errOut = %q, err = %v", r.out, r.errOut, r.err)
	}
	r = runCmd(t, imageAPI(), nil, "", "image", "delete", testID, "--yes")
	if r.err != nil || r.out != "Deleted "+testID+"\n" {
		t.Errorf("delete: out = %q, err = %v", r.out, r.err)
	}
}

func TestSearch(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "cat dog" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `[{"image_id":"a","permalink_url":"https://gyazo.com/a"}]`)
	})
	r := runCmd(t, api, nil, "", "search", "cat", "dog", "--as", "permalink")
	if r.err != nil || r.out != "https://gyazo.com/a\n" {
		t.Errorf("out = %q, err = %v", r.out, r.err)
	}
}

func TestErrors(t *testing.T) {
	never := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
	})
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"invalid output", []string{"image", "list", "-o", "xml"}, "unknown output format"},
		{"invalid image", []string{"image", "get", "https://example.com/x"}, "not a gyazo.com URL"},
		{"invalid link style", []string{"upload", "a.png", "--as", "html"}, "unknown link style"},
		{"invalid access policy", []string{"upload", "a.png", "--access-policy", "public"}, "invalid access policy"},
		{"stdin twice", []string{"upload", "-", "-"}, "only once"},
		{"per page", []string{"image", "list", "--per-page", "101"}, "--per-page"},
		{"search limit", []string{"search", "x", "--page", "101", "--per", "100"}, "10000"},
		{"one bad arg aborts all", []string{"image", "delete", testID, "bad/id", "--yes"}, "invalid image ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runCmd(t, never, nil, "", tt.args...)
			if r.err == nil || !strings.Contains(r.err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", r.err, tt.want)
			}
		})
	}
}

func TestErrorHint(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	})
	r := runCmd(t, api, nil, "", "search", "x")
	if hint := ErrorHint(r.err); !strings.Contains(hint, "Pro") {
		t.Errorf("hint = %q for %v", hint, r.err)
	}
}

func TestAuthLoginStatusLogout(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	env := map[string]string{config.EnvConfigFile: cfgPath, config.EnvAccessToken: ""}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer goodtoken123" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"This method requires authentication"}`)
			return
		}
		fmt.Fprint(w, `{"user":{"name":"alice","email":"a@example.com","uid":"u1"}}`)
	})

	if r := runCmd(t, api, env, "badtoken\n", "auth", "login", "--with-token"); r.err == nil {
		t.Fatal("expected verification error for invalid token")
	}
	if f, _ := config.ReadFile(cfgPath); !f.Credentials.IsZero() {
		t.Fatal("invalid token must not be saved")
	}

	if r := runCmd(t, api, env, "goodtoken123\n", "auth", "login", "--with-token"); r.err != nil {
		t.Fatal(r.err)
	}
	if f, _ := config.ReadFile(cfgPath); f.Credentials.AccessToken != "goodtoken123" {
		t.Fatalf("credentials = %+v", f.Credentials)
	}

	r := runCmd(t, api, env, "", "auth", "status", "-o", "json")
	if r.err != nil {
		t.Fatal(r.err)
	}
	var st authStatus
	if err := json.Unmarshal([]byte(r.out), &st); err != nil {
		t.Fatal(err)
	}
	if st.User != "alice" || st.Source != config.SourceFile || strings.Contains(r.out, "goodtoken123") {
		t.Errorf("unexpected status (token must be masked): %s", r.out)
	}

	if r := runCmd(t, api, env, "", "whoami"); r.err != nil || r.out != "alice (a@example.com)\n" {
		t.Errorf("whoami: out = %q, err = %v", r.out, r.err)
	}

	if r := runCmd(t, api, env, "", "auth", "logout"); r.err != nil {
		t.Fatal(r.err)
	}
	if f, _ := config.ReadFile(cfgPath); !f.Credentials.IsZero() {
		t.Errorf("credentials remain after logout: %+v", f.Credentials)
	}
}

func TestConfigSetGet(t *testing.T) {
	env := map[string]string{config.EnvConfigFile: filepath.Join(t.TempDir(), "config.json")}
	if r := runCmd(t, nil, env, "", "config", "set", "upload.access-policy", "only_me"); r.err != nil {
		t.Fatal(r.err)
	}
	if r := runCmd(t, nil, env, "", "config", "get", "upload.access-policy"); r.out != "only_me\n" {
		t.Errorf("get = %q, err = %v", r.out, r.err)
	}
	if r := runCmd(t, nil, env, "", "config", "set", "upload.access-policy", "public"); r.err == nil {
		t.Error("expected validation error")
	}
	if r := runCmd(t, nil, env, "", "config", "set", "nokey", "v"); r.err == nil {
		t.Error("expected error for unknown key")
	}
}
