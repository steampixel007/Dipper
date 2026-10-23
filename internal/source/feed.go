package source

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

type rssDocument struct {
	XMLName  xml.Name     `xml:"rss"`
	Version  string       `xml:"version,attr"`
	Channels []rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Link  string    `xml:"link"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	Published string `xml:"pubDate"`
	// Mikan has used both mikanime.tv and mikanani.me extension namespaces;
	// encoding/xml matches these fields by local name rather than a fixed host.
	Torrent struct {
		Link      string `xml:"link"`
		Length    string `xml:"contentLength"`
		Published string `xml:"pubDate"`
	} `xml:"torrent"`
	Enclosure struct {
		URL    string `xml:"url,attr"`
		Length string `xml:"length,attr"`
		Type   string `xml:"type,attr"`
	} `xml:"enclosure"`
}

// Feed returns the current RSS window (at most 100 entries on Mikan), not complete
// history. Use History for subscription initialization and backfills.
func (c *Client) Feed(ctx context.Context, animeID, groupID string) ([]Resource, error) {
	if !numericID.MatchString(animeID) || !numericID.MatchString(groupID) {
		return nil, fmt.Errorf("mikan: invalid anime or subtitle group ID")
	}
	endpoint, err := c.endpoint("/RSS/Bangumi", url.Values{"bangumiId": {animeID}, "subgroupid": {groupID}})
	if err != nil {
		return nil, err
	}
	body, base, err := c.get(ctx, endpoint, maxDocumentBytes)
	if err != nil {
		return nil, err
	}
	var feed rssDocument
	decoder := xml.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&feed); err != nil {
		return nil, fmt.Errorf("%w: malformed RSS: %v", ErrStructure, err)
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: malformed RSS trailer", ErrStructure)
		}
		if text, ok := token.(xml.CharData); ok && strings.TrimSpace(string(text)) == "" {
			continue
		}
		if _, ok := token.(xml.Comment); !ok {
			return nil, fmt.Errorf("%w: unexpected RSS trailer", ErrStructure)
		}
	}
	if feed.Version != "2.0" || len(feed.Channels) != 1 {
		return nil, fmt.Errorf("%w: missing RSS 2.0 channel", ErrStructure)
	}
	channel := feed.Channels[0]
	link, err := url.Parse(strings.TrimSpace(channel.Link))
	if err != nil || clean(channel.Title) == "" || queryValue(link, "bangumiId") != animeID || queryValue(link, "subgroupid") != groupID {
		return nil, fmt.Errorf("%w: missing or mismatched RSS identity", ErrStructure)
	}
	items := make([]Resource, 0, len(channel.Items))
	seen := make(map[string]bool, len(channel.Items))
	for _, item := range channel.Items {
		id, err := idFromURL(strings.TrimSpace(item.Link), "/Home/Episode/")
		id = strings.ToLower(id)
		if err != nil || !hashID.MatchString(id) || seen[id] || clean(item.Title) == "" {
			return nil, fmt.Errorf("%w: malformed or duplicate RSS episode", ErrStructure)
		}
		if item.Torrent.Link != "" {
			otherID, err := idFromURL(strings.TrimSpace(item.Torrent.Link), "/Home/Episode/")
			if err != nil || !strings.EqualFold(otherID, id) {
				return nil, fmt.Errorf("%w: mismatched RSS episode link", ErrStructure)
			}
		}
		torrent, err := absoluteURL(base, item.Enclosure.URL)
		if err != nil || !torrentMatches(torrent, id) || item.Enclosure.Type != "application/x-bittorrent" {
			return nil, fmt.Errorf("%w: missing or invalid RSS torrent enclosure", ErrStructure)
		}
		length := strings.TrimSpace(item.Enclosure.Length)
		if length == "" {
			length = strings.TrimSpace(item.Torrent.Length)
		}
		size, err := strconv.ParseInt(length, 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("%w: invalid RSS content length", ErrStructure)
		}
		if item.Torrent.Length != "" {
			otherSize, err := strconv.ParseInt(strings.TrimSpace(item.Torrent.Length), 10, 64)
			if err != nil || size != otherSize {
				return nil, fmt.Errorf("%w: mismatched RSS content lengths", ErrStructure)
			}
		}
		published := strings.TrimSpace(item.Torrent.Published)
		if published == "" {
			published = strings.TrimSpace(item.Published)
		}
		if published == "" {
			return nil, fmt.Errorf("%w: missing RSS publication date", ErrStructure)
		}
		seen[id] = true
		// RSS supplies a source ID and torrent URL, not a verified BT identity.
		// The download layer must read the torrent before confirming its infohash.
		items = append(items, Resource{ID: id, Title: strings.TrimSpace(item.Title), Published: published,
			TorrentURL: torrent, Size: size})
	}
	return items, nil
}

func queryValue(u *url.URL, key string) string {
	for name, values := range u.Query() {
		if strings.EqualFold(name, key) && len(values) == 1 {
			return values[0]
		}
	}
	return ""
}
