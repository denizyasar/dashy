package server

import _ "embed"

//go:embed assets/index.html
var IndexHtml string

//go:embed assets/dashboard.html
var DashboardHtml string

//go:embed assets/history.html
var HistoryHtml string

//go:embed assets/bulma.css
var BulmaCSS string

//go:embed assets/fontawesome.css
var FontAwesomeCSS string

//go:embed assets/main.css
var MainCSS string
