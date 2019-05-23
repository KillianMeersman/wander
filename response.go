package wander

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// Response is a wrapper around http.Response as well as an already parsed Goquery document
type Response struct {
	Request *Request
	*http.Response
	*goquery.Document
}

func NewResponse(req *Request, res *http.Response) (*Response, error) {
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	return &Response{
		req,
		res,
		doc,
	}, err
}
