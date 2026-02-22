package handler

import "mycli.sh/api/internal/email"

// mockEmailSender is a test double for email.Sender.
type mockEmailSender struct {
	SendVerificationFn func(params email.EmailParams) error
	calls              []email.EmailParams
}

func (m *mockEmailSender) SendVerification(params email.EmailParams) error {
	m.calls = append(m.calls, params)
	if m.SendVerificationFn != nil {
		return m.SendVerificationFn(params)
	}
	return nil
}
