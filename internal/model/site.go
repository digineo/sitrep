package model

// Route modes of a site.
const (
	RoutePath      = "path"
	RouteSubdomain = "subdomain"
	RouteCustom    = "custom"
)

// Site is a public status page.
type Site struct {
	ID        string    `json:"id"`
	Name      Text      `json:"name"`
	Languages Languages `json:"languages"`
	Route     Route     `json:"route"`
}

// Route says how a site is reached: below a base domain path, as subdomain
// of every base domain, or on its own domain.
type Route struct {
	Mode   string `json:"mode"`
	Slug   string `json:"slug,omitempty"`
	Domain string `json:"domain,omitempty"`
}
