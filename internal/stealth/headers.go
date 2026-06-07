package stealth

// HeaderSet is a consistent header bundle plus advisories the MCP cannot enforce.
type HeaderSet struct {
	Headers    map[string]string `json:"headers"`
	Advisories []string          `json:"advisories"`
}

// HeadersFor builds an HTTP header set consistent with the given fingerprint.
func HeadersFor(fp Fingerprint) HeaderSet {
	return HeaderSet{
		Headers: map[string]string{
			"User-Agent":         fp.UserAgent,
			"Accept-Language":    fp.Locale + ",en;q=0.9",
			"Sec-CH-UA":          fp.SecCHUA,
			"Sec-CH-UA-Platform": fp.SecCHUAPlatform,
			"Sec-CH-UA-Mobile":   "?0",
			"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		},
		Advisories: []string{
			"TLS/JA3 fingerprint is set by the browser's TLS stack and cannot be changed by this MCP — use a matching browser build or a TLS-rotating proxy.",
			"HTTP/2 frame fingerprint is browser-controlled; header order here is advisory only.",
		},
	}
}
