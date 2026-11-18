package mirrors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	catalogURL = "https://xmirror.voidlinux.org/v0/mirrors.json"
	statusURL  = "https://prometheus.voidlinux.org/api/v1/query?query=(time()-repo_origin_time%7Bzone=%22external%22%7D)"
	defaultURL = "https://repo-default.voidlinux.org/"
	maxBody    = 1 << 20
)

type Mirror struct {
	URL      string
	Region   string
	Location string
	Tier     int
	Age      time.Duration
	Status   string
}

type Catalog struct {
	Mirrors  []Mirror
	Warnings []string
}

type Client struct {
	httpClient *http.Client
	catalogURL string
	statusURL  string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		catalogURL: catalogURL,
		statusURL:  statusURL,
	}
}

func (c *Client) List(ctx context.Context) (Catalog, error) {
	var entries []struct {
		URL      string `json:"base_url"`
		Region   string `json:"region"`
		Location string `json:"location"`
		Tier     int    `json:"tier"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.getJSON(ctx, c.catalogURL, &entries); err != nil {
		return Catalog{}, fmt.Errorf("load Void mirror catalog: %w", err)
	}

	ages, err := c.loadAges(ctx)
	catalog := Catalog{}
	if err != nil {
		catalog.Warnings = []string{"Live mirror status is temporarily unavailable."}
	}
	for _, entry := range entries {
		if !entry.Enabled || entry.URL == "" || (entry.Tier != 1 && entry.Tier != 2) {
			continue
		}
		age, reachable := ages[normalizeURL(entry.URL)]
		catalog.Mirrors = append(catalog.Mirrors, Mirror{URL: entry.URL, Region: regionName(entry.Region), Location: entry.Location, Tier: entry.Tier, Age: age, Status: mirrorStatus(age, reachable)})
	}
	if !containsMirror(catalog.Mirrors, defaultURL) {
		catalog.Mirrors = append(catalog.Mirrors, Mirror{URL: defaultURL, Region: "Global", Location: "Void Linux default redirector", Tier: 1, Status: "Healthy"})
	}
	return catalog, nil
}

func containsMirror(items []Mirror, url string) bool {
	wanted := normalizeURL(url)
	for _, item := range items {
		if normalizeURL(item.URL) == wanted {
			return true
		}
	}
	return false
}

func (c *Client) loadAges(ctx context.Context) (map[string]time.Duration, error) {
	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric struct {
					Instance string `json:"instance"`
				} `json:"metric"`
				Value []json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, c.statusURL, &response); err != nil {
		return nil, err
	}
	if response.Status != "success" {
		return nil, fmt.Errorf("status response was %q", response.Status)
	}
	ages := make(map[string]time.Duration, len(response.Data.Result))
	for _, result := range response.Data.Result {
		if len(result.Value) != 2 {
			continue
		}
		var seconds string
		if json.Unmarshal(result.Value[1], &seconds) != nil {
			continue
		}
		value, err := time.ParseDuration(seconds + "s")
		if err == nil {
			ages[normalizeURL(strings.TrimSuffix(result.Metric.Instance, "/current"))] = value
		}
	}
	return ages, nil
}

func (c *Client) getJSON(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(target)
}

func normalizeURL(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "https://"), "http://")
	return strings.TrimSuffix(value, "/")
}

func mirrorStatus(age time.Duration, reachable bool) string {
	if !reachable {
		return "Unreachable"
	}
	if age >= 24*time.Hour {
		return "Stale"
	}
	if age >= 19*time.Hour {
		return "Delayed"
	}
	return "Healthy"
}

func regionName(code string) string {
	regions := map[string]string{"AF": "Africa", "AN": "Antarctica", "AS": "Asia", "EU": "Europe", "NA": "North America", "OC": "Oceania", "SA": "South and Central America", "World": "Global"}
	if name := regions[code]; name != "" {
		return name
	}
	return code
}
