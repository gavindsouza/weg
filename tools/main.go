package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var tools = []Tool{
	{
		Name:       "devbox",
		CheckCmd:   "devbox",
		InstallURL: "https://github.com/jetify-com/devbox/releases/latest/download/devbox_%s_%s.tar.gz",
		BinName:    "devbox",
		Archive:    "tar.gz",
	},
	{
		Name:       "direnv",
		CheckCmd:   "direnv",
		InstallURL: "https://github.com/direnv/direnv/releases/latest/download/direnv.%s-%s",
		BinName:    "direnv",
		Archive:    "", // raw binary
	},
}

type Tool struct {
	Name       string
	CheckCmd   string
	InstallURL string
	BinName    string
	Archive    string
}

type FrappeApp struct {
	Name   string
	Url    string
	Branch string
}

func EnsureToolsInstalled() {
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.CheckCmd); err != nil {
			fmt.Printf("🔧 %s not found. Installing...\n", tool.Name)
			if err := installTool(tool); err != nil {
				log.Fatalf("❌ Failed to install %s: %v", tool.Name, err)
			}
			fmt.Printf("✅ Installed %s successfully.\n", tool.Name)
		} else {
			fmt.Printf("✅ %s is already installed.\n", tool.Name)
		}
	}
}

func installTool(tool Tool) error {
	osType := runtime.GOOS
	arch := runtime.GOARCH
	url := fmt.Sprintf(tool.InstallURL, osType, arch)

	localBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	os.MkdirAll(localBin, 0755)
	targetPath := filepath.Join(localBin, tool.BinName)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if tool.Archive == "tar.gz" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		tr := tar.NewReader(gr)

		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if strings.HasSuffix(hdr.Name, "/"+tool.BinName) || hdr.Name == tool.BinName {
				out, err := os.Create(targetPath)
				if err != nil {
					return err
				}
				defer out.Close()
				if _, err := io.Copy(out, tr); err != nil {
					return err
				}
				break
			}
		}
	} else if strings.HasSuffix(url, ".zip") {
		tmpFile, err := os.CreateTemp("", tool.BinName+".zip")
		if err != nil {
			return err
		}
		defer os.Remove(tmpFile.Name())

		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return err
		}
		tmpFile.Close()

		r, err := zip.OpenReader(tmpFile.Name())
		if err != nil {
			return err
		}
		defer r.Close()

		for _, f := range r.File {
			if f.Name == tool.BinName {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				defer rc.Close()

				out, err := os.Create(targetPath)
				if err != nil {
					return err
				}
				defer out.Close()

				_, err = io.Copy(out, rc)
				if err != nil {
					return err
				}
			}
		}
	} else {
		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, resp.Body); err != nil {
			return err
		}
	}

	if err := os.Chmod(targetPath, 0755); err != nil {
		return err
	}

	pathSet := strings.Contains(os.Getenv("PATH"), localBin)
	if !pathSet {
		fmt.Printf("⚠️  Please add %s to your PATH manually.\n", localBin)
	}

	return nil
}

func CloneRepos(repos []FrappeApp, baseDir string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))

	for _, repo := range repos {
		wg.Add(1)
		go func(r FrappeApp) {
			defer wg.Done()
			target := filepath.Join(baseDir, r.Name)

			// Skip if already cloned
			if _, err := os.Stat(target); err == nil {
				return
			}

			cmd := exec.Command("git", "clone", "--depth=1", r.Url, target)
			output, err := cmd.CombinedOutput()
			if err != nil {
				errChan <- fmt.Errorf("failed to clone %s: %v\n%s", r.Url, err, string(output))
			}
		}(repo)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		for err := range errChan {
			fmt.Fprintln(os.Stderr, err)
		}
		return fmt.Errorf("one or more clones failed")
	}

	return nil
}
