package email

const Type = "send_email"

type Payload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type Message struct {
	To      string
	Subject string
	Body    string
}
