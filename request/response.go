package request

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// Response holds the original Request, as well as the http Response and goquery document.
// Response instances can be searched by using qoquery methods.
type Response struct {
	http.Response
	Request  *Request
	Document *goquery.Document
}

// Parse the document in a document, caches the document in the document field.
func (r *Response) Parse() (*goquery.Document, error) {
	if r.Document != nil {
		return r.Document, nil
	}

	doc, err := goquery.NewDocumentFromResponse(&r.Response)
	if err != nil {
		return nil, err
	}
	r.Document = doc
	return doc, nil
}

// NewResponse returns a Response. Returns an error if the response body could not be parsed by goquery.
func NewResponse(req *Request, res http.Response) *Response {
	return &Response{
		res,
		req,
		nil,
	}
}
