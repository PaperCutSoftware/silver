package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCheckUpdate_HitUpdateService_Ok(t *testing.T) {
	//Arrange
	h := &SuccessResponseHandler{}
	server := httptest.NewServer(h)
	server.URL += "/check-update"
	defer server.Close()

	//Act
	checkURL := server.URL
	info, err := checkUpdate(checkURL, "2015-09-16-0758")

	//Assert
	if err != nil {
		t.Fatalf("got an error: %v", err)
	}

	expectInfo := &UpgradeInfo{
		URL:     "downloadURL",
		Version: "2016-01-01-1212",
		Sha256:  "sha256string",
		Operations: []Operation{
			{
				Action: "move", Args: []string{"update2016-09-16-0758", "v2016-09-16-0758"},
			},
		},
	}
	if !reflect.DeepEqual(info, expectInfo) {
		t.Fatalf("Got different info: expect: %#v, actual: %#v", expectInfo, info)
	}
}

func TestCheckUpdate_ReceiveCacheControlHeadersFromRequest(t *testing.T) {
	//Arrange
	h := &RequestHeadCheckHandler{}
	server := httptest.NewServer(h)
	server.URL += "/check-update"
	defer server.Close()

	//Act
	checkURL := server.URL
	_, err := checkUpdate(checkURL, "2015-09-16-0758")

	//Assert
	if err != nil {
		t.Fatalf("expected no error but got an error: %v", err)
	}
}

func TestCheckUpdate_RequestExtraQueryParametersWithVersion(t *testing.T) {
	//Arrange
	h := &RequestQueryStringCheckHandler{}
	server := httptest.NewServer(h)
	server.URL += "/check-update?platform=win&other1=value1&other2=value2"
	defer server.Close()

	//Act
	checkURL := server.URL
	_, err := checkUpdate(checkURL, "2015-09-16-0758")

	//Assert
	if err != nil {
		t.Fatalf("expected no error but got an error: %v", err)
	}
}

type RequestHeadCheckHandler struct{}

func (h *RequestHeadCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	cacheControl := r.Header.Get("Cache-Control")
	pragma := r.Header.Get("Pragma")
	expires := r.Header.Get("Expires")
	if cacheControl == "" || pragma == "" || expires == "" {
		http.Error(w, "Missing some cache control headers", http.StatusBadRequest)
		return
	}
	(&SuccessResponseHandler{}).ServeHTTP(w, r)
}

type RequestQueryStringCheckHandler struct{}

func (h *RequestQueryStringCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	platform := r.URL.Query().Get("platform")
	other1 := r.URL.Query().Get("other1")
	other2 := r.URL.Query().Get("other2")
	if platform == "" || other1 == "" || other2 == "" {
		http.Error(w, "Missing some query parameters", http.StatusBadRequest)
		return
	}

	(&SuccessResponseHandler{}).ServeHTTP(w, r)
}

type SuccessResponseHandler struct{}

func (h *SuccessResponseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fmt.Printf("URL: %v\n", r.URL)

	version := r.URL.Query().Get("version")
	if version == "" {
		http.Error(w, "Missing version in URL query string", http.StatusBadRequest)
		return
	}

	res := `{
		"url": "downloadURL",
		"version": "2016-01-01-1212",
		"sha256": "sha256string",
		"operations": [
			{"action": "move", "args": ["update2016-09-16-0758", "v2016-09-16-0758"]}
		]
	}`
	w.Write([]byte(res))
}
