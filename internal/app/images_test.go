package app

import (
	"bytes"
	"mime"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func imageMultipartBody(t *testing.T, model string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", model); err != nil {
		t.Fatal(err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("png-data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func TestImageEditFields(t *testing.T) {
	body, contentType := imageMultipartBody(t, "image-model")
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	model, hasFile, err := imageEditFields(body, params["boundary"])
	if err != nil || model != "image-model" || !hasFile {
		t.Fatalf("fields = model %q, file %v, err %v", model, hasFile, err)
	}
}

func TestReadImageBodyAcceptsConfiguredLargerLimit(t *testing.T) {
	body := strings.Repeat("x", 50<<20+1)
	s := &Service{cfg: Config{ImageMaxBodyBytes: 51 << 20}}
	got, err := s.readImageBody(httptest.NewRecorder(), httptest.NewRequest("POST", "/v1/images/edits", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("readImageBody returned error: %v", err)
	}
	if len(got) != len(body) {
		t.Fatalf("body length = %d, want %d", len(got), len(body))
	}
}

func TestRewriteMultipartModel(t *testing.T) {
	body, contentType := imageMultipartBody(t, "public-model")
	rewritten, rewrittenType := rewriteMultipartModel(body, contentType, "upstream-model")
	model, hasFile, err := func() (string, bool, error) {
		_, params, err := mime.ParseMediaType(rewrittenType)
		if err != nil {
			return "", false, err
		}
		return imageEditFields(rewritten, params["boundary"])
	}()
	if err != nil || model != "upstream-model" || !hasFile {
		t.Fatalf("rewritten fields = model %q, file %v, err %v", model, hasFile, err)
	}
	if strings.Contains(string(rewritten), "public-model") {
		t.Fatal("public model leaked into rewritten multipart body")
	}
}
