package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltpl "html/template"
	"log"
	texttpl "text/template"

	"github.com/resend/resend-go/v3"
)

//go:embed templates/*.html templates/*.txt
var templateFS embed.FS

var (
	htmlTmpl = htmltpl.Must(htmltpl.ParseFS(templateFS, "templates/magic_link.html"))
	textTmpl = texttpl.Must(texttpl.ParseFS(templateFS, "templates/magic_link.txt"))
)

type EmailParams struct {
	To        string
	VerifyURL string
	OTPCode   string // empty if no OTP
}

type Sender interface {
	SendVerification(params EmailParams) error
}

// ResendSender sends emails via the Resend SDK.
type ResendSender struct {
	client *resend.Client
	from   string
}

func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

func (s *ResendSender) SendVerification(params EmailParams) error {
	htmlBody, textBody, err := renderTemplates(params)
	if err != nil {
		return err
	}

	_, err = s.client.Emails.Send(&resend.SendEmailRequest{
		From:    s.from,
		To:      []string{params.To},
		Subject: "mycli — Verify your email",
		Html:    htmlBody,
		Text:    textBody,
	})
	if err != nil {
		return fmt.Errorf("resend send email: %w", err)
	}
	return nil
}

// LogSender prints verification details to stdout for development.
type LogSender struct{}

func (s *LogSender) SendVerification(params EmailParams) error {
	log.Printf("[DEV] Magic link for %s: %s", params.To, params.VerifyURL)
	if params.OTPCode != "" {
		log.Printf("[DEV] OTP code for %s: %s", params.To, params.OTPCode)
	}
	return nil
}

func renderTemplates(params EmailParams) (html, text string, err error) {
	data := struct {
		VerifyURL string
		OTPCode   string
	}{
		VerifyURL: params.VerifyURL,
		OTPCode:   params.OTPCode,
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", fmt.Errorf("render html template: %w", err)
	}

	var textBuf bytes.Buffer
	if err := textTmpl.Execute(&textBuf, data); err != nil {
		return "", "", fmt.Errorf("render text template: %w", err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}
