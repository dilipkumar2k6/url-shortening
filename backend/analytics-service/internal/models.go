package internal

type ClickEvent struct {
	ShortCode string  `json:"short_code"`
	Timestamp string  `json:"timestamp"`
	Country   *string `json:"country,omitempty"`
	UserAgent string  `json:"user_agent"`
}

type TopLinkStats struct {
	ShortCode   string `json:"short_code" ch:"short_code"`
	LongURL     string `json:"long_url"`
	TotalClicks uint64 `json:"total_clicks" ch:"clicks"`
}

type TopLinksResponse struct {
	Links []TopLinkStats `json:"top_links"`
}
