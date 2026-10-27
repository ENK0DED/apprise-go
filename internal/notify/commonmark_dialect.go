package notify

func commonMarkToGoogleChat(body string) string { return commonMarkToAnchors(body, true) }
func commonMarkToSlack(body string) string      { return commonMarkToAnchors(body, false) }
func commonMarkToWhatsApp(body string) string   { return commonMarkToWhatsAppScanner(body) }
