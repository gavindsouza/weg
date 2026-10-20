/*
Copyright © 2025 Gavin <me@gavv.in>

The site's time zone, which Frappe's naive datetimes are expressed in.
*/
package remote

import (
	"fmt"
	"strings"
	"time"

	// Embed the zone database so a site's zone resolves on hosts without one.
	_ "time/tzdata"
)

// GetSystemTimeZone returns the site's System Settings time zone (an IANA
// name such as "Europe/Berlin"), or "" when the site has none set.
func (c *Client) GetSystemTimeZone() (string, error) {
	msg, err := c.CallMethod("frappe.client.get_single_value", map[string]any{
		"doctype": "System Settings",
		"field":   "time_zone",
	})
	if err != nil {
		return "", err
	}
	tz, _ := msg.(string)
	return strings.TrimSpace(tz), nil
}

// SiteLocation resolves the site's time zone. Frappe stores datetimes such as
// Version.creation as naive wall-clock time in this zone. When it cannot be
// read or parsed, SiteLocation returns UTC together with an error describing
// why, so the caller can warn that times may be off by the site's offset.
func (f *Fetcher) SiteLocation() (*time.Location, error) {
	name, err := f.Client.GetSystemTimeZone()
	if err != nil {
		return time.UTC, fmt.Errorf("could not read System Settings time zone: %w", err)
	}
	return resolveLocation(name)
}

// frappeDefaultTimeZone is the zone Frappe itself uses when System Settings
// has none (frappe.utils.get_system_timezone).
const frappeDefaultTimeZone = "Asia/Kolkata"

// resolveLocation maps a System Settings time zone name to a location. An
// empty name means Frappe's own default; an unknown one falls back to UTC
// with an error.
func resolveLocation(name string) (*time.Location, error) {
	if name == "" {
		name = frappeDefaultTimeZone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC, fmt.Errorf("unknown site time zone %q: %w", name, err)
	}
	return loc, nil
}
