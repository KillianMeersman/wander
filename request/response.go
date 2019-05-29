package request

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// Response holds the original Request, as well as the http Response and goquery document.
// Response instances can be searched by using qoquery methods.
type Response struct {
	Request *Request
	*http.Response
	*goquery.Document
}

// NewResponse returns a Response. Returns an error if the response body could not be parsed by goquery.
func NewResponse(req *Request, res *http.Response) (*Response, error) {
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	return &Response{
		req,
		res,
		doc,
	}, err
}
