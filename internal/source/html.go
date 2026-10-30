package source

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const maxDocumentBytes int64 = 64 << 20

var (
	numericID = regexp.MustCompile(`^[1-9][0-9]*$`)
	hashID    = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	cssURL    = regexp.MustCompile(`(?i)url\(\s*['"]?([^'"\)]+)['"]?\s*\)`)
	sizeText  = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*(B|KB|MB|GB|TB|KIB|MIB|GIB|TIB)$`)
)

func (c *Client) document(ctx context.Context, path string, query url.Values) (*goquery.Document, *url.URL, error) {
	endpoint, err := c.endpoint(path, query)
	if err != nil {
		return nil, nil, err
	}
	body, base, err := c.get(ctx, endpoint, maxDocumentBytes)
	if err != nil {
		return nil, nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrStructure, err)
	}
	// HTML parsing repairs truncated tables, which could otherwise look like a
	// legitimate short final history response.
	if doc.Find(".episode-table").Length() > 0 && !bytes.Contains(bytes.ToLower(body), []byte("</table>")) {
		return nil, nil, fmt.Errorf("%w: unterminated episode table", ErrStructure)
	}
	return doc, base, nil
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func absoluteURL(base *url.URL, raw string) (string, error) {
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%w: missing or invalid URL", ErrStructure)
	}
	u := base.ResolveReference(ref)
	if _, err := httpURL(u.String()); err != nil {
		return "", fmt.Errorf("%w: %v", ErrStructure, err)
	}
	return u.String(), nil
}

func idFromURL(raw, prefix string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(strings.ToLower(u.Path), strings.ToLower(prefix)) {
		return "", fmt.Errorf("%w: invalid %s link", ErrStructure, prefix)
	}
	return strings.TrimSuffix(u.Path[len(prefix):], "/"), nil
}

// Catalog takes a season as the source site writes it (the four Chinese season
// characters) or as its English name. The Chinese characters are the site's
// own vocabulary and stay untranslated.
func (c *Client) Catalog(ctx context.Context, year int, season string) ([]Anime, error) {
	seasons := map[string]string{"冬": "冬", "春": "春", "夏": "夏", "秋": "秋", "winter": "冬", "spring": "春", "summer": "夏", "autumn": "秋", "fall": "秋"}
	s, ok := seasons[strings.ToLower(strings.TrimSpace(season))]
	if !ok || year < 1900 || year > 9999 {
		return nil, fmt.Errorf("mikan: invalid catalog year or season")
	}
	doc, base, err := c.document(ctx, "/Home/BangumiCoverFlowByDayOfWeek", url.Values{"year": {strconv.Itoa(year)}, "seasonStr": {s}})
	if err != nil {
		return nil, err
	}
	sections := doc.Find(".sk-bangumi[data-dayofweek]")
	if sections.Length() == 0 {
		return nil, fmt.Errorf("%w: missing catalog sections", ErrStructure)
	}
	items := make([]Anime, 0)
	seen := make(map[string]bool)
	var parseErr error
	sections.EachWithBreak(func(_ int, section *goquery.Selection) bool {
		day := section.AttrOr("data-dayofweek", "")
		if len(day) != 1 || day[0] < '0' || day[0] > '8' {
			parseErr = fmt.Errorf("%w: invalid catalog day", ErrStructure)
			return false
		}
		if day == "7" || day == "8" {
			day = ""
		}
		section.Find(".an-ul > li").EachWithBreak(func(_ int, li *goquery.Selection) bool {
			poster := li.Find("[data-bangumiid][data-src]").First()
			id := poster.AttrOr("data-bangumiid", "")
			title := clean(li.Find(".an-text").First().Text())
			if title == "" {
				title = clean(li.Find(".an-info-group [title]").First().AttrOr("title", ""))
			}
			cover, coverErr := absoluteURL(base, poster.AttrOr("data-src", ""))
			if !numericID.MatchString(id) || title == "" || coverErr != nil {
				parseErr = fmt.Errorf("%w: malformed catalog entry %q", ErrStructure, id)
				return false
			}
			if link := li.Find("a.an-text"); link.Length() > 0 {
				linkedID, err := idFromURL(link.AttrOr("href", ""), "/Home/Bangumi/")
				if err != nil || linkedID != id {
					parseErr = fmt.Errorf("%w: catalog ID mismatch", ErrStructure)
					return false
				}
			}
			if seen[id] {
				return true
			}
			seen[id] = true
			items = append(items, Anime{ID: id, Title: title, Cover: cover, Day: day})
			return true
		})
		return parseErr == nil
	})
	if parseErr != nil {
		return nil, parseErr
	}
	// An empty section is valid; a populated card container with no recognized
	// cards indicates changed markup rather than an empty seasonal catalog.
	if len(items) == 0 && doc.Find(".an-box li, [data-bangumiid]").Length() > 0 {
		return nil, fmt.Errorf("%w: unrecognized catalog cards", ErrStructure)
	}
	return items, nil
}

func (c *Client) Detail(ctx context.Context, animeID string) (Detail, error) {
	if !numericID.MatchString(animeID) {
		return Detail{}, fmt.Errorf("mikan: invalid anime ID")
	}
	doc, base, err := c.document(ctx, "/Home/Bangumi/"+animeID, nil)
	if err != nil {
		return Detail{}, err
	}
	title := doc.Find(".bangumi-title").First()
	rssURL, err := url.Parse(title.Find("a.mikan-rss").AttrOr("href", ""))
	if err != nil || rssURL.Query().Get("bangumiId") != animeID || clean(title.Text()) == "" {
		return Detail{}, fmt.Errorf("%w: missing or mismatched anime identity", ErrStructure)
	}
	style := doc.Find(".bangumi-poster").First().AttrOr("style", "")
	match := cssURL.FindStringSubmatch(style)
	if len(match) != 2 {
		return Detail{}, fmt.Errorf("%w: missing anime cover", ErrStructure)
	}
	cover, err := absoluteURL(base, match[1])
	if err != nil {
		return Detail{}, err
	}
	result := Detail{Anime: Anime{ID: animeID, Title: clean(title.Text()), Cover: cover,
		Summary: clean(doc.Find(".central-container > .header2-desc, .bangumi-summary, .bangumi-description").First().Text())}, Groups: make([]Group, 0)}
	doc.Find(".bangumi-info").Each(func(_ int, info *goquery.Selection) {
		if text := clean(info.Text()); strings.HasPrefix(text, "放送日期：") {
			result.Anime.Day = weekday(strings.TrimSpace(strings.TrimPrefix(text, "放送日期：")))
		}
	})
	seen := make(map[string]bool)
	groupNames := make(map[string]string)
	doc.Find(".leftbar-nav .subgroup-name").Each(func(_ int, group *goquery.Selection) {
		groupNames[strings.TrimPrefix(group.AttrOr("data-anchor", ""), "#")] = clean(group.Text())
	})
	var parseErr error
	doc.Find(".central-container .subgroup-text").EachWithBreak(func(_ int, header *goquery.Selection) bool {
		id := header.AttrOr("id", "")
		name := groupNames[id]
		if name == "" {
			name = clean(header.ChildrenFiltered("a[href^='/Home/PublishGroup/']").First().Text())
		}
		if !numericID.MatchString(id) || name == "" || seen[id] {
			parseErr = fmt.Errorf("%w: malformed or duplicate subtitle group %q (%q)", ErrStructure, id, name)
			return false
		}
		table := header.Next()
		resources, err := parseTable(table, base)
		if err != nil {
			parseErr = err
			return false
		}
		count := len(resources)
		expand := table.NextFiltered(".js-expand-episode")
		if expand.Length() > 0 {
			if expand.AttrOr("data-bangumiid", "") != animeID || expand.AttrOr("data-subtitlegroupid", "") != id {
				parseErr = fmt.Errorf("%w: expansion group mismatch", ErrStructure)
				return false
			}
			count = -1
		}
		seen[id] = true
		result.Groups = append(result.Groups, Group{ID: id, Name: name, Count: count})
		return true
	})
	if parseErr != nil {
		return Detail{}, parseErr
	}
	if doc.Find(".leftbar-nav").Length() == 0 || doc.Find(".central-container").Length() == 0 {
		return Detail{}, fmt.Errorf("%w: missing subtitle group containers", ErrStructure)
	}
	for _, node := range doc.Find(".leftbar-nav .subgroup-name").EachIter() {
		if !seen[strings.TrimPrefix(node.AttrOr("data-anchor", ""), "#")] {
			return Detail{}, fmt.Errorf("%w: missing subtitle group table", ErrStructure)
		}
	}
	return result, nil
}

func weekday(text string) string {
	days := map[string]string{"星期日": "0", "星期天": "0", "星期一": "1", "星期二": "2", "星期三": "3", "星期四": "4", "星期五": "5", "星期六": "6"}
	return days[text]
}

// History retrieves successively larger whole tables. A partial result is never
// returned on failure or if the bounded take limit is reached without exhaustion.
func (c *Client) History(ctx context.Context, animeID, groupID string) ([]Resource, error) {
	if !numericID.MatchString(animeID) || !numericID.MatchString(groupID) {
		return nil, fmt.Errorf("mikan: invalid anime or subtitle group ID")
	}
	var previous []Resource
	for take := c.initialTake; take <= c.maxTake; take *= 2 {
		doc, base, err := c.document(ctx, "/Home/ExpandEpisodeTable", url.Values{
			"bangumiId": {animeID}, "subtitleGroupId": {groupID}, "take": {strconv.Itoa(take)},
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrIncompleteHistory, err)
		}
		items, err := parseTable(doc.Find(".episode-table"), base)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrIncompleteHistory, err)
		}
		if len(items) > take || !containsHistory(items, previous) {
			return nil, fmt.Errorf("%w: history regressed or exceeded requested take", ErrIncompleteHistory)
		}
		if len(items) == 0 {
			// The endpoint returns the same empty table for an unknown group as
			// for a valid empty group; check the detail page before accepting it.
			detail, err := c.Detail(ctx, animeID)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrIncompleteHistory, err)
			}
			for _, group := range detail.Groups {
				if group.ID == groupID && group.Count == 0 {
					return items, nil
				}
				if group.ID == groupID {
					return nil, fmt.Errorf("%w: empty response for a nonempty group", ErrIncompleteHistory)
				}
			}
			return nil, ErrNotFound
		}
		if len(items) < take {
			return items, nil
		}
		previous = items
	}
	return nil, fmt.Errorf("%w: take limit %d reached", ErrIncompleteHistory, c.maxTake)
}

func containsHistory(items, previous []Resource) bool {
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.ID] = true
	}
	for _, item := range previous {
		if !ids[item.ID] {
			return false
		}
	}
	return true
}

func parseTable(container *goquery.Selection, base *url.URL) ([]Resource, error) {
	if container.Length() != 1 || !container.HasClass("episode-table") || container.Find("table").Length() != 1 || container.Find("table > tbody").Length() != 1 {
		return nil, fmt.Errorf("%w: missing episode table", ErrStructure)
	}
	items := make([]Resource, 0)
	seen := make(map[string]bool)
	var parseErr error
	container.Find("table > tbody > tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		link := row.Find("a.magnet-link-wrap")
		id, err := idFromURL(link.AttrOr("href", ""), "/Home/Episode/")
		id = strings.ToLower(id)
		title := strings.TrimSpace(link.Text())
		if err != nil || link.Length() != 1 || !hashID.MatchString(id) || title == "" || seen[id] {
			parseErr = fmt.Errorf("%w: malformed or duplicate episode %q", ErrStructure, id)
			return false
		}
		magnet := row.Find("[data-clipboard-text^='magnet:']").First().AttrOr("data-clipboard-text", "")
		if magnet == "" {
			magnet = row.Find("[data-magnet]").First().AttrOr("data-magnet", "")
		}
		m, magnetErr := url.Parse(magnet)
		if magnetErr != nil || m.Scheme != "magnet" || !strings.EqualFold(m.Query().Get("xt"), "urn:btih:"+id) {
			parseErr = fmt.Errorf("%w: missing or mismatched episode magnet", ErrStructure)
			return false
		}
		cell := link.Closest("td")
		size, sizeErr := parseSize(clean(cell.Next().Text()))
		published := clean(cell.Next().Next().Text())
		torrent, torrentErr := absoluteURL(base, row.Find("a[href*='/Download/']").First().AttrOr("href", ""))
		if sizeErr != nil || published == "" || torrentErr != nil || !torrentMatches(torrent, id) {
			parseErr = fmt.Errorf("%w: episode %s has invalid size %q, date %q or torrent %q", ErrStructure, id, clean(cell.Next().Text()), published, torrent)
			return false
		}
		seen[id] = true
		items = append(items, Resource{ID: id, Title: title, Published: published, Size: size, Magnet: magnet, TorrentURL: torrent})
		return true
	})
	if parseErr != nil {
		return nil, parseErr
	}
	return items, nil
}

func torrentMatches(raw, id string) bool {
	u, err := url.Parse(raw)
	return err == nil && strings.HasSuffix(strings.ToLower(u.Path), "/"+id+".torrent")
}

func parseSize(text string) (int64, error) {
	// Some older entries have an empty size cell; zero means not supplied.
	if text == "" {
		return 0, nil
	}
	match := sizeText.FindStringSubmatch(text)
	if len(match) != 3 {
		return 0, fmt.Errorf("invalid source size %q", text)
	}
	n, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	unit := strings.ToUpper(match[2])
	if unit != "B" {
		n *= math.Pow(1024, float64(strings.Index("KMGT", unit[:1])+1))
	}
	if math.IsInf(n, 0) || n >= math.MaxInt64 {
		return 0, fmt.Errorf("source size overflow")
	}
	return int64(math.Round(n)), nil
}
