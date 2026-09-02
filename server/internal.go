package server

import (
	"fmt"
	"html/template"
	"os"
)

var (
	TIMESTAMP_FORMAT   = "02/01/2006 15:04:05"
	INDEX_TEMPLATE     = loadTemplate("/index.html", IndexHtml)
	DASHBOARD_TEMPLATE = loadTemplate("/dashboard.html", DashboardHtml)
	HISTORY_TEMPLATE   = loadTemplate("/history.html", HistoryHtml)
)

func loadTemplate(path string, templateString string) *template.Template {
	templ, err := template.New(path).Parse(templateString)
	if err != nil {
		fmt.Println("error: " + err.Error())
		os.Exit(1)
	}
	return templ
}

type Status string

const (
	StatusSuccess Status = "success"
	StatusDanger  Status = "danger"
	StatusNeutral Status = "grey"
)
