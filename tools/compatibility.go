package tools

import (
	"fmt"
	"regexp"
)

type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type FrappeVersion struct {
	Version        string       `json:"version"`
	VersionPattern string       `json:"versionPattern"`
	Dependencies   []Dependency `json:"dependencies"`
}

type Frappe struct {
	Versions []FrappeVersion `json:"versions"`
}

var frappe = Frappe{
	Versions: []FrappeVersion{
		{
			Version:        "14.x.x",
			VersionPattern: `^(v?14\.|version-14(-.*)?)`,
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
			Version:        "15.x.x",
			VersionPattern: `^(v?15\.|version-15(-.*)?)`,
			Dependencies: []Dependency{
				{Name: "python", Version: "3.10"},
				{Name: "nodejs", Version: "18"},
				{Name: "redis", Version: "6.2"},
				{Name: "mariadb", Version: "10.6"},
				{Name: "wkhtmltopdf", Version: "0.12.6"},
			},
		},
		{
			Version:        "develop",
			VersionPattern: `^develop$`,
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
		matched, _ := regexp.MatchString(v.VersionPattern, version)
		if matched {
			return v.Dependencies, nil
		}
	}
	return nil, fmt.Errorf("version %s not found", version)
}
