package mirrors

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}
}

func TestListCombinesEnabledMirrorsWithStatus(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/catalog" {
			return response(http.StatusOK, `[{"base_url":"https://fresh.example/","region":"EU","location":"Test","tier":1,"enabled":true},{"base_url":"https://off.example/","region":"NA","location":"Disabled","tier":2,"enabled":false}]`), nil
		}
		return response(http.StatusOK, `{"status":"success","data":{"result":[{"metric":{"instance":"fresh.example/current"},"value":[0,"3600"]}]}}`), nil
	})}
	client := &Client{httpClient: httpClient, catalogURL: "https://test.invalid/catalog", statusURL: "https://test.invalid/status"}

	got, err := client.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Mirrors) != 2 || got.Mirrors[0].Status != "Healthy" || got.Mirrors[0].Age != time.Hour || got.Mirrors[0].Region != "Europe" || got.Mirrors[1].URL != "https://repo-default.voidlinux.org/" {
		t.Fatalf("unexpected catalog: %#v", got)
	}
}

func TestListKeepsCatalogWhenStatusFails(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/catalog" {
			return response(http.StatusOK, `[{"base_url":"https://mirror.example/","region":"AS","location":"Test","tier":2,"enabled":true}]`), nil
		}
		return response(http.StatusBadGateway, "down"), nil
	})}
	client := &Client{httpClient: httpClient, catalogURL: "https://test.invalid/catalog", statusURL: "https://test.invalid/status"}

	got, err := client.List(context.Background())
	if err != nil || len(got.Mirrors) != 2 || got.Mirrors[0].Status != "Unreachable" || len(got.Warnings) != 1 {
		t.Fatalf("catalog = %#v, err = %v", got, err)
	}
}
