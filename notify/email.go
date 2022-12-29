package notify

import (
	"es-backup/config"
	"net/smtp"
	"strconv"
)

func Email(params *config.Params, subject string, message string, isError bool) {
	if !params.Notify.Email.Enabled {
		return
	}

	var smtpHost, from, password, to string
	var smtpPort int

	if isError {
		smtpHost = params.Notify.Email.Error.SmtpHost
		smtpPort = params.Notify.Email.Error.SmtpPort
		from = params.Notify.Email.Error.From
		password = params.Notify.Email.Error.Password
		to = params.Notify.Email.Error.To
	} else {
		smtpHost = params.Notify.Email.Info.SmtpHost
		smtpPort = params.Notify.Email.Info.SmtpPort
		from = params.Notify.Email.Info.From
		password = params.Notify.Email.Info.Password
		to = params.Notify.Email.Info.To
	}

	auth := smtp.PlainAuth("", from, password, smtpHost)

	msg := []byte("From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: [" + params.Hostname + "] " + subject + "\r\n\r\n" +
		message + "\r\n")

	_ = smtp.SendMail(smtpHost+":"+strconv.Itoa(smtpPort), auth, from, []string{to}, msg)
}
