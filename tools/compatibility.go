package tools

import "fmt"

type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type FrappeVersion struct {
	Version      string       `json:"version"`
	Dependencies []Dependency `json:"dependencies"`
}

type Frappe struct {
	Versions []FrappeVersion `json:"versions"`
}

var frappe = Frappe{
	Versions: []FrappeVersion{
		{
			Version: "14.x.x",
			Dependencies: []Dependency{
				{Name: "python", Version: "3.10"},
				{Name: "nodejs", Version: "16"},
				{Name: "redis", Version: "6.2"},
				{Name: "mariadb", Version: "10.6"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
				// {Name: "libfontconfig1", Version: "2.13.1"},
				// {Name: "libxrender1", Version: "1:0.9.10"},
				// {Name: "libxext6", Version: "1:1.3.4"},
			},
		},
		{
			Version: "15.x.x",
			Dependencies: []Dependency{
				{Name: "python3.10", Version: "3.10"},
				{Name: "nodejs", Version: "18"},
				{Name: "redis-server", Version: "6.2"},
				{Name: "mariadb-server", Version: "10.6"},
				{Name: "wkhtmltopdf", Version: "0.12.6-1"},
			},
		},
		{
			Version: "develop",
			Dependencies: []Dependency{
				{Name: "python", Version: "3.13"},
				{Name: "nodejs", Version: "22"},
				{Name: "redis", Version: "7"},
				{Name: "mariadb", Version: "10.6"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
				{Name: "pnpm"},
			},
		},
	},
}

func GetDependencies(version string) ([]Dependency, error) {
	for _, v := range frappe.Versions {
		if v.Version == version {
			return v.Dependencies, nil
		}
	}
	return nil, fmt.Errorf("version %s not found", version)
}
