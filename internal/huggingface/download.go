package huggingface

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const HFAPI = "https://huggingface.co"

type Downloader struct {
	Token  string
	Client *http.Client
}

func NewDownloader(token string) *Downloader {
	return &Downloader{
		Token:  token,
		Client: &http.Client{},
	}
}

func DownloadURL(repo, filename string) string {
	return fmt.Sprintf("%s/%s/resolve/main/%s", HFAPI, repo, filename)
}

type ProgressFunc func(downloaded, total int64)

func (d *Downloader) DownloadFile(repo, filename, destDir string, progressFn ProgressFunc) error {
	destPath := filepath.Join(destDir, filename)

	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	url := DownloadURL(repo, filename)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if d.Token != "" {
		req.Header.Set("Authorization", "Bearer "+d.Token)
	}

	var existingSize int64
	partialPath := destPath + ".partial"
	if info, err := os.Stat(partialPath); err == nil {
		existingSize = info.Size()
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed for %s: %w", filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf(
			"access denied for %s/%s\n\n"+
				"This is a gated model. To download:\n"+
				"  1. Accept the license at https://huggingface.co/%s\n"+
				"  2. Set your token: export HF_TOKEN=hf_xxx\n"+
				"     Or use: voicaa pull %s --token hf_xxx",
			repo, filename, repo, filepath.Base(repo),
		)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected HTTP %d for %s/%s", resp.StatusCode, repo, filename)
	}

	totalSize := resp.ContentLength + existingSize

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	flags := os.O_CREATE | os.O_WRONLY
	if existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		existingSize = 0
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(partialPath, flags, 0644)
	if err != nil {
		return err
	}

	downloaded := existingSize
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				return writeErr
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded, totalSize)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			return readErr
		}
	}
	f.Close()

	return os.Rename(partialPath, destPath)
}

func ExtractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		// Guard against zip-slip
		if !isSubPath(destDir, target) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}

func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && rel[:2] != ".."
}
