package spider

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

type Response struct {
	*http.Response
	*goquery.Document
}

func NewResponse(res *http.Response) (*Response, error) {
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	return &Response{
		res,
		doc,
	}, err
}
