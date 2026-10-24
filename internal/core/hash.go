package core

import (
	"context"
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dipper.local/dipper/internal/proxy"
)

func canonicalHash(s string) (string, error) {
	if len(s) == 40 {
		b, err := hex.DecodeString(s)
		if err == nil {
			return hex.EncodeToString(b), nil
		}
	}
	if len(s) == 32 {
		b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(s))
		if err == nil && len(b) == 20 {
			return hex.EncodeToString(b), nil
		}
	}
	return "", errors.New("invalid BT info hash")
}

// ResourceHash returns the v1 info hash when the resource already provides one.
// A torrent with an opaque URL is resolved later, after its intent is committed.
func ResourceHash(r Resource) (string, error) {
	if r.Magnet != "" {
		u, err := url.Parse(r.Magnet)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(u.Scheme, "magnet") {
			return "", errors.New("invalid magnet scheme")
		}
		var hash string
		for _, xt := range u.Query()["xt"] {
			if strings.HasPrefix(strings.ToLower(xt), "urn:btih:") {
				h, err := canonicalHash(xt[len("urn:btih:"):])
				if err != nil {
					return "", err
				}
				if hash != "" && h != hash {
					return "", errors.New("magnet has conflicting info hashes")
				}
				hash = h
			}
		}
		if hash != "" {
			return hash, nil
		}
		return "", errors.New("magnet has no v1 BT info hash")
	}
	return "", errors.New("resource requires torrent info hash resolution")
}

func resolveHash(ctx context.Context, r Resource) (string, error) {
	if hash, err := ResourceHash(r); err == nil {
		return hash, nil
	}
	if r.TorrentURL == "" {
		return "", errors.New("resource has neither a valid magnet nor a torrent URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.TorrentURL, nil)
	if err != nil {
		return "", err
	}
	client := http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{Proxy: proxy.Func},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("torrent HTTP status %d", resp.StatusCode)
	}
	const maxTorrent = 8 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxTorrent+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxTorrent {
		return "", errors.New("torrent exceeds 8 MiB")
	}
	return torrentHash(b)
}

func torrentHash(b []byte) (string, error) {
	if len(b) == 0 || b[0] != 'd' {
		return "", errors.New("invalid torrent dictionary")
	}
	p := 1
	var info []byte
	for p < len(b) && b[p] != 'e' {
		key, end, err := bencodeString(b, p)
		if err != nil {
			return "", err
		}
		p = end
		end, err = bencodeEnd(b, p, 0)
		if err != nil {
			return "", err
		}
		if key == "info" {
			if info != nil || b[p] != 'd' {
				return "", errors.New("invalid torrent info dictionary")
			}
			info = b[p:end]
		}
		p = end
	}
	if p >= len(b) || p+1 != len(b) || len(info) == 0 {
		return "", errors.New("torrent has no valid info dictionary")
	}
	h := sha1.Sum(info)
	return hex.EncodeToString(h[:]), nil
}

func bencodeString(b []byte, p int) (string, int, error) {
	if p >= len(b) || b[p] < '0' || b[p] > '9' {
		return "", 0, errors.New("invalid bencode string")
	}
	start := p
	for p < len(b) && b[p] >= '0' && b[p] <= '9' {
		p++
	}
	if p >= len(b) || b[p] != ':' {
		return "", 0, errors.New("invalid bencode string length")
	}
	n, err := strconv.Atoi(string(b[start:p]))
	if err != nil || n < 0 || n > len(b)-p-1 {
		return "", 0, errors.New("bencode string out of bounds")
	}
	return string(b[p+1 : p+1+n]), p + 1 + n, nil
}

func bencodeEnd(b []byte, p, depth int) (int, error) {
	if p >= len(b) || depth > 100 {
		return 0, errors.New("invalid bencode nesting")
	}
	switch b[p] {
	case 'i':
		end := p + 1
		for end < len(b) && b[end] != 'e' {
			end++
		}
		if end >= len(b) {
			return 0, errors.New("unterminated bencode integer")
		}
		if _, err := strconv.ParseInt(string(b[p+1:end]), 10, 64); err != nil {
			return 0, err
		}
		return end + 1, nil
	case 'l', 'd':
		dict := b[p] == 'd'
		p++
		for p < len(b) && b[p] != 'e' {
			if dict {
				_, end, err := bencodeString(b, p)
				if err != nil {
					return 0, err
				}
				p = end
			}
			end, err := bencodeEnd(b, p, depth+1)
			if err != nil {
				return 0, err
			}
			p = end
		}
		if p >= len(b) {
			return 0, errors.New("unterminated bencode collection")
		}
		return p + 1, nil
	default:
		_, end, err := bencodeString(b, p)
		return end, err
	}
}
