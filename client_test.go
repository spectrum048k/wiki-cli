package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAzureDevOpsClientUsesDefaultBaseURL(t *testing.T) {
	client := NewAzureDevOpsClient("my-org", "token")
	if client.baseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("base URL = %q, want %q", client.baseURL, "https://dev.azure.com/my-org")
	}
}

func TestAzureDevOpsClientWithBaseURLUsesServerCollectionPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/tfs/DefaultCollection/MyProject/_apis/wiki/wikis" {
			t.Errorf("request path = %q", request.URL.Path)
		}
		response.Write([]byte(`{"value":[]}`))
	}))
	defer server.Close()

	client := NewAzureDevOpsClientWithBaseURL("DefaultCollection", "token", server.URL+"/tfs/DefaultCollection/")
	client.httpClient = server.Client()
	if _, err := client.ListWikis("MyProject"); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeAzureDevOpsBaseURL(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "https://ado.example.com/tfs/DefaultCollection/", want: "https://ado.example.com/tfs/DefaultCollection"},
		{input: "http://localhost:8080/collection", want: "http://localhost:8080/collection"},
		{input: "ado.example.com/collection", wantErr: true},
		{input: "ftp://ado.example.com/collection", wantErr: true},
		{input: "https://user:password@ado.example.com/collection", wantErr: true},
		{input: "https://ado.example.com/collection?query=1", wantErr: true},
		{input: "https://ado.example.com/collection#fragment", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeAzureDevOpsBaseURL(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeAzureDevOpsBaseURL(%q) succeeded; want error", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeAzureDevOpsBaseURL(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizeAzureDevOpsBaseURL(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
