package robots

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

type SitemapLocation struct {
	Loc        string    `xml:"loc"`
	LastMod    time.Time `xml:"lastmod"`
	ChangeFreq string    `xml:"changefreq"`
	Priority   float64   `xml:"priority"`
}

type Sitemap struct {
	Index  []SitemapLocation `xml:"sitemap"`
	URLSet []SitemapLocation `xml:"url"`
}

func NewSitemap() *Sitemap {
	return &Sitemap{
		Index:  make([]SitemapLocation, 0),
		URLSet: make([]SitemapLocation, 0),
	}
}

func NewSitemapFromReader(reader io.Reader) (*Sitemap, error) {
	var sitemap Sitemap
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	err = xml.Unmarshal(data, &sitemap)
	if err != nil {
		return nil, err
	}

	return &sitemap, nil
}

func NewSitemapFromURL(url string, client http.RoundTripper) (*Sitemap, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.RoundTrip(request)
	defer res.Body.Close()
	return NewSitemapFromReader(res.Body)
}

// GetLocations gets up to <limit> sitemap locations.
// Sitemaps usually come in pages of 50k entries, this means the limit may be exceeded by up to 49_999 entries.
func (s *Sitemap) GetLocations(client http.RoundTripper, limit int) ([]SitemapLocation, error) {
	urls := s.URLSet

	for _, index := range s.Index {
		if len(urls) >= limit {
			break
		}

		// make request
		request, err := http.NewRequest("GET", index.Loc, nil)
		if err != nil {
			return nil, err
		}
		res, err := client.RoundTrip(request)
		defer res.Body.Close()

		// unmarshal sitemap
		var sitemap Sitemap
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		err = xml.Unmarshal(data, &sitemap)
		if err != nil {
			return nil, err
		}

		urls = append(urls, sitemap.URLSet...)
	}

	return urls, nil
}
