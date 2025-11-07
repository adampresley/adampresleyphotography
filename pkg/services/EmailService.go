package services

import (
	"html/template"
	"strings"

	"github.com/adampresley/adamgokit/email"
)

func SendEmail(apiKey, toName, toEmail, fromName, fromEmail string, data map[string]any) error {
	parsedTemplate := strings.Builder{}

	service := email.NewResendService(&email.Config{
		ApiKey: apiKey,
	})

	tmpl := `
<h1>Your photo album is ready!</h1>
<p>Hello {{.toName}}! The photos download you requested is now
ready. You can click the button below to download the album '{{.albumName}}'
as a ZIP file containing your photos. This link will expire in 2 days.</p>
<a href="{{.downloadURL}}">Download Album</a>
	`

	data["toName"] = toName

	t := template.Must(template.New("email").Parse(tmpl))
	_ = t.Execute(&parsedTemplate, data)

	return service.Send(email.Mail{
		Body:       parsedTemplate.String(),
		BodyIsHtml: true,
		From: email.EmailAddress{
			Email: fromEmail,
			Name:  fromName,
		},
		Subject: "Your photos download is ready!",
		To: []email.EmailAddress{
			{Name: toName, Email: toEmail},
		},
	})
}
