package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AzureDevOpsError struct{ Message string }

func (err *AzureDevOpsError) Error() string { return err.Message }

type AzureDevOpsClient struct {
	organization string
	token        string
	apiVersion   string
	baseURL      string
	httpClient   *http.Client
}

func NewAzureDevOpsClient(organization, token string) *AzureDevOpsClient {
	return newAzureDevOpsClient(organization, token, "https://dev.azure.com/"+url.PathEscape(organization))
}

func newAzureDevOpsClient(organization, token, baseURL string) *AzureDevOpsClient {
	return &AzureDevOpsClient{
		organization: organization,
		token:        token,
		apiVersion:   "7.1",
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (client *AzureDevOpsClient) request(target, accept string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+client.token)))
	request.Header.Set("Accept", accept)
	return client.httpClient.Do(request)
}

func (client *AzureDevOpsClient) get(apiPath string, params url.Values, accept string) (*http.Response, error) {
	if params == nil {
		params = make(url.Values)
	}
	params.Set("api-version", client.apiVersion)
	target := client.baseURL + apiPath + "?" + params.Encode()
	var response *http.Response
	for attempt := 0; attempt < 4; attempt++ {
		var err error
		response, err = client.request(target, accept)
		if err != nil {
			return nil, err
		}
		if !retryableStatus(response.StatusCode) || attempt == 3 {
			break
		}
		delay := time.Duration(1<<attempt) * time.Second
		if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds >= 0 {
			delay = time.Duration(seconds) * time.Second
		}
		response.Body.Close()
		time.Sleep(delay)
	}
	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 200))
		response.Body.Close()
		return nil, &AzureDevOpsError{Message: fmt.Sprintf("Azure DevOps request failed (%d): %s", response.StatusCode, string(body))}
	}
	return response, nil
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (client *AzureDevOpsClient) ListWikis(project string) ([]Wiki, error) {
	response, err := client.get("/"+url.PathEscape(project)+"/_apis/wiki/wikis", nil, "application/json")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var payload struct {
		Value []Wiki `json:"value"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Value, nil
}

func (client *AzureDevOpsClient) GetPage(project string, wiki Wiki, pagePath string) (Page, error) {
	params := url.Values{"path": {pagePath}, "recursionLevel": {"Full"}}
	response, err := client.get("/"+url.PathEscape(project)+"/_apis/wiki/wikis/"+url.PathEscape(wiki.ID)+"/pages", params, "application/json")
	if err != nil {
		return Page{}, err
	}
	defer response.Body.Close()
	var page Page
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		return Page{}, err
	}
	if etag := response.Header.Get("ETag"); etag != "" {
		page.ETag = &etag
	}
	return page, nil
}

func (client *AzureDevOpsClient) GetRawPage(project string, wiki Wiki, pagePath string) (string, *string, error) {
	params := url.Values{"path": {pagePath}}
	response, err := client.get("/"+url.PathEscape(project)+"/_apis/wiki/wikis/"+url.PathEscape(wiki.ID)+"/pages", params, "text/plain")
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, err
	}
	var etag *string
	if value := response.Header.Get("ETag"); value != "" {
		etag = &value
	}
	return string(content), etag, nil
}

func (client *AzureDevOpsClient) DownloadBinary(target string) ([]byte, error) {
	response, err := client.request(target, "*/*")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Failed to download attachment (%d): %s", response.StatusCode, target)
	}
	return io.ReadAll(response.Body)
}

func (client *AzureDevOpsClient) Organization() string { return client.organization }
