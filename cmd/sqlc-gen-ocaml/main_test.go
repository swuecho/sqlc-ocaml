package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestJSONProtocol(t *testing.T) {
	in := `{"settings":{"engine":"postgresql"},"catalog":{"schemas":[]},"queries":[],"plugin_options":"` + base64.StdEncoding.EncodeToString([]byte(`{"filename":"db"}`)) + `"}`
	var out bytes.Buffer
	if err := run(bytes.NewBufferString(in), &out); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Files []struct {
			Name     string `json:"name"`
			Contents []byte `json:"contents"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Files) != 2 || response.Files[0].Name != "db.ml" {
		t.Fatalf("unexpected response: %s", out.String())
	}
}

func TestJSONProtocolRejectsTrailingValue(t *testing.T) {
	input := `{"settings":{"engine":"postgresql"}} {}`
	if err := run(bytes.NewBufferString(input), &bytes.Buffer{}); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}
