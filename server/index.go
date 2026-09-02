package server

import (
	"net/http"
)

type Index struct {
	Links []DashboardLink
}

type DashboardLink struct {
	Name string
	Url  string
}

func (index *Index) Render(w http.ResponseWriter) {
	INDEX_TEMPLATE.Execute(w, index)
}
