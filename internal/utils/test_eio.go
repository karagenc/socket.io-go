package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func EIOHandshake(t *testing.T, ts *httptest.Server) (sid string) {
	url, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := url.Query()
	q.Add("transport", "polling")
	q.Add("EIO", "4")
	url.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = body[1:]
	m := make(map[string]interface{})
	err = json.Unmarshal(body, &m)
	if err != nil {
		t.Fatal(err)
	}
	_sid, ok := m["sid"]
	if !ok {
		t.Fatal("!ok")
	}
	sid, ok = _sid.(string)
	if !ok {
		t.Fatal("not a string")
	}
	return
}

func EIOPush(t *testing.T, ts *httptest.Server, sid, body string) {
	url, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := url.Query()
	q.Add("transport", "polling")
	q.Add("EIO", "4")
	q.Add("sid", sid)
	url.RawQuery = q.Encode()

	bodyBuf := bytes.NewBufferString(body)
	req, err := http.NewRequest("POST", url.String(), bodyBuf)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func EIOPoll(t *testing.T, ts *httptest.Server, sid string) (body string, status int) {
	url, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := url.Query()
	q.Add("transport", "polling")
	q.Add("EIO", "4")
	q.Add("sid", sid)
	url.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(respBytes), resp.StatusCode
}
