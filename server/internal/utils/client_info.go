package utils

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mssola/user_agent"
)

type ClientLocation struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type ClientInfo struct {
	OS         string
	Browser    string
	DeviceName string
	City       string
	Country    string
}

// httpClient for fast IP lookups without hanging the auth flow
var ipClient = &http.Client{
	Timeout: 2 * time.Second,
}

// ParseClientInfo extracts OS, browser, device from UA, and city/country from IP.
func ParseClientInfo(ip, userAgent string) ClientInfo {
	info := ClientInfo{
		OS:         "Unknown",
		Browser:    "Unknown",
		DeviceName: "Unknown",
		City:       "Unknown",
		Country:    "Unknown",
	}

	// 1. Parse User Agent
	if userAgent != "" {
		ua := user_agent.New(userAgent)
		
		info.OS = ua.OS()
		name, version := ua.Browser()
		if name != "" {
			info.Browser = name + " " + version
		}
		
		if ua.Mobile() {
			info.DeviceName = "Mobile"
		} else if ua.Bot() {
			info.DeviceName = "Bot"
		} else {
			info.DeviceName = "Desktop"
		}
	}

	// 2. Parse IP (Skip for local loopback)
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		info.City = "Local Network"
		info.Country = "Local Network"
		return info
	}

	// Fast GeoIP lookup (fail-safe)
	// Using ip-api.com (free, no key needed, fast)
	req, err := http.NewRequest("GET", "http://ip-api.com/json/"+ip+"?fields=city,country", nil)
	if err == nil {
		if resp, err := ipClient.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				var loc ClientLocation
				if err := json.NewDecoder(resp.Body).Decode(&loc); err == nil {
					if loc.City != "" {
						info.City = loc.City
					}
					if loc.Country != "" {
						info.Country = loc.Country
					}
				}
			}
		}
	}

	return info
}
