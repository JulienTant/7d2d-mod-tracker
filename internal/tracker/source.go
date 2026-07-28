package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	nexusURLPattern = regexp.MustCompile(`(?i)nexusmods\.com/([^/]+)/mods/([0-9]+)`)
	nexusIDPattern  = regexp.MustCompile(`^[0-9]+$`)
	slugPattern     = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	pagePatterns    = []*regexp.Regexp{
		regexp.MustCompile(`(?i)"version"\s*:\s*"([^"]+)"`),
		regexp.MustCompile(`(?i)"softwareVersion"\s*:\s*"([^"]+)"`),
		regexp.MustCompile(`(?is)>\s*Version\s*</[^>]+>\s*(?:<[^>]+>\s*){0,3}v?([0-9]+(?:\.[0-9]+){0,3})`),
		regexp.MustCompile(`(?i)\bVersion\s+v?([0-9]+(?:\.[0-9]+){0,3})`),
	}
)

type CheckResult struct {
	URL     string
	Version string
	Err     error
}

type SourceIDs struct {
	Nexus    *string `json:"nexus"`
	SevenD2D *string `json:"7d2dmods"`
}

func (ids SourceIDs) URLs() []string {
	var result []string
	if ids.Nexus != nil && *ids.Nexus != "" {
		result = append(result, "https://www.nexusmods.com/7daystodie/mods/"+*ids.Nexus)
	}
	if ids.SevenD2D != nil && *ids.SevenD2D != "" {
		result = append(result, "https://7daystodiemods.com/mods/"+*ids.SevenD2D+"/")
	}
	return result
}

func SourceIDsFromURLs(values []string) SourceIDs {
	var ids SourceIDs
	for _, value := range values {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if (host == "nexusmods.com" || strings.HasSuffix(host, ".nexusmods.com")) && len(parts) >= 3 {
			for i := 0; i+1 < len(parts); i++ {
				if parts[i] == "mods" {
					id := parts[i+1]
					ids.Nexus = &id
					break
				}
			}
		}
		if host == "7daystodiemods.com" || strings.HasSuffix(host, ".7daystodiemods.com") {
			for i := 0; i+1 < len(parts); i++ {
				if parts[i] == "mods" {
					slug := parts[i+1]
					ids.SevenD2D = &slug
					break
				}
			}
		}
	}
	return ids
}

func (ids SourceIDs) NexusValue() string {
	if ids.Nexus == nil {
		return ""
	}
	return *ids.Nexus
}

func (ids SourceIDs) SevenD2DValue() string {
	if ids.SevenD2D == nil {
		return ""
	}
	return *ids.SevenD2D
}

func SourceIDsFromInputs(nexusInput, sevenD2DInput string) (SourceIDs, error) {
	var ids SourceIDs
	nexusInput = strings.TrimSpace(nexusInput)
	if nexusInput != "" {
		if nexusIDPattern.MatchString(nexusInput) {
			id := nexusInput
			ids.Nexus = &id
		} else {
			parsed := SourceIDsFromURLs([]string{nexusInput})
			if parsed.Nexus == nil {
				return ids, fmt.Errorf("Nexus source must be a numeric mod ID or Nexus Mods URL")
			}
			ids.Nexus = parsed.Nexus
		}
	}

	sevenD2DInput = strings.TrimSpace(strings.Trim(sevenD2DInput, "/"))
	if sevenD2DInput != "" {
		if slugPattern.MatchString(sevenD2DInput) {
			slug := sevenD2DInput
			ids.SevenD2D = &slug
		} else {
			parsed := SourceIDsFromURLs([]string{sevenD2DInput})
			if parsed.SevenD2D == nil {
				return ids, fmt.Errorf("7 Days to Die Mods source must be a mod slug or URL")
			}
			ids.SevenD2D = parsed.SevenD2D
		}
	}
	return ids, nil
}

type Checker struct {
	Client *http.Client
	APIKey string
}

func IsSupportedSource(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "nexusmods.com" || strings.HasSuffix(host, ".nexusmods.com") ||
		host == "7daystodiemods.com" || strings.HasSuffix(host, ".7daystodiemods.com")
}

func NewChecker(apiKey string) *Checker {
	return &Checker{Client: &http.Client{Timeout: 20 * time.Second}, APIKey: apiKey}
}

func sevenD2DAPIURL(pageURL string) string {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "7daystodiemods.com" && !strings.HasSuffix(host, ".7daystodiemods.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "mods" || parts[1] == "" {
		return ""
	}
	return "https://api.7daystodiemods.com/v1/mods/" + url.PathEscape(parts[1])
}

func extractSevenD2DVersion(body []byte) string {
	var payload struct {
		CurrentVersion *struct {
			Version string `json:"version"`
		} `json:"current_version"`
		ModFiles []struct {
			Version string `json:"mod_version"`
		} `json:"mod_files"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	if payload.CurrentVersion != nil && VersionIdentifier(payload.CurrentVersion.Version) != "" {
		return VersionIdentifier(payload.CurrentVersion.Version)
	}
	for _, file := range payload.ModFiles {
		if version := VersionIdentifier(file.Version); version != "" {
			return version
		}
	}
	return ""
}

func (c *Checker) Check(ctx context.Context, pageURL string) CheckResult {
	if !IsSupportedSource(pageURL) {
		return CheckResult{URL: pageURL, Err: fmt.Errorf("unsupported update source")}
	}
	targetURL := pageURL
	sevenD2DAPI := false
	headers := map[string]string{"Accept": "text/html,application/json"}
	if apiURL := sevenD2DAPIURL(pageURL); apiURL != "" {
		targetURL = apiURL
		sevenD2DAPI = true
		headers["Accept"] = "application/json"
	} else if match := nexusURLPattern.FindStringSubmatch(pageURL); len(match) == 3 && c.APIKey != "" {
		targetURL = fmt.Sprintf("https://api.nexusmods.com/v1/games/%s/mods/%s.json", match[1], match[2])
		headers["apikey"] = c.APIKey
		headers["Accept"] = "application/json"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return CheckResult{URL: pageURL, Err: err}
	}
	request.Header.Set("User-Agent", "7D2D-Mod-Tracker/0.1")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := c.Client.Do(request)
	if err != nil {
		return CheckResult{URL: pageURL, Err: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return CheckResult{URL: pageURL, Err: fmt.Errorf("HTTP %d", response.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return CheckResult{URL: pageURL, Err: err}
	}
	if sevenD2DAPI {
		version := extractSevenD2DVersion(body)
		if version == "" {
			return CheckResult{URL: pageURL, Err: ErrNoVersion}
		}
		return CheckResult{URL: pageURL, Version: version}
	}
	if targetURL != pageURL {
		var payload struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Version != "" {
			return CheckResult{URL: pageURL, Version: payload.Version}
		}
	}
	version := extractPageVersion(string(body))
	if version == "" {
		return CheckResult{URL: pageURL, Err: ErrNoVersion}
	}
	return CheckResult{URL: pageURL, Version: version}
}

func extractPageVersion(page string) string {
	for _, pattern := range pagePatterns {
		matches := pattern.FindAllStringSubmatch(page, -1)
		if len(matches) == 0 {
			continue
		}
		counts := make(map[string]int)
		var versions []string
		for _, match := range matches {
			version := VersionFromText(match[1])
			if version == "" {
				version = strings.TrimSpace(match[1])
			}
			counts[version]++
			versions = append(versions, version)
		}
		sort.Slice(versions, func(i, j int) bool {
			if counts[versions[i]] != counts[versions[j]] {
				return counts[versions[i]] > counts[versions[j]]
			}
			return compareParts(VersionParts(versions[i]), VersionParts(versions[j])) > 0
		})
		return versions[0]
	}
	return ""
}

func compareParts(left, right []int) int {
	for i := 0; i < max(len(left), len(right)); i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}
	return 0
}

func Latest(results []CheckResult) CheckResult {
	var valid []CheckResult
	for _, result := range results {
		if result.Version != "" {
			valid = append(valid, result)
		}
	}
	if len(valid) == 0 {
		return CheckResult{Err: ErrNoVersion}
	}
	sort.Slice(valid, func(i, j int) bool {
		return compareParts(VersionParts(valid[i].Version), VersionParts(valid[j].Version)) > 0
	})
	return valid[0]
}
