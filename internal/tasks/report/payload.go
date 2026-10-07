package report

const Type = "generate_report"

type Payload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Document struct {
	JobID   string
	Title   string
	Content string
}
