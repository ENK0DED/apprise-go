package notify

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestAnchorMarkdownUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/anchor_markdown.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Input, Slack, Chat, WhatsApp, Telegram1, Telegram2 string }
	if err = json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for i, v := range vectors {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for name, c := range map[string]struct{ got, want string }{"slack": {commonMarkToSlack(v.Input), v.Slack}, "chat": {commonMarkToGoogleChat(v.Input), v.Chat}, "whatsapp": {commonMarkToWhatsApp(v.Input), v.WhatsApp}, "telegram1": {commonMarkToTelegram(v.Input, false), v.Telegram1}, "telegram2": {commonMarkToTelegram(v.Input, true), v.Telegram2}} {
				if c.got != c.want {
					t.Errorf("%s input %q\nGo %q\nPython %q", name, v.Input, c.got, c.want)
				}
			}
		})
	}
}
