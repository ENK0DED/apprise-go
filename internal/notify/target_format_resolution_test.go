package notify

import "testing"

func TestTargetFormatResolution(t *testing.T) {
	for _, tc := range []struct{ name, url, input, want string }{
		{"single-url-override", "ntfy://topic?format=markdown", "text", "markdown"},
		{"single-default", "ntfy://topic", "markdown", "text"},
		{"multi-input", "slack://T111/B222/token", "text", "text"},
		{"multi-explicit", "slack://T111/B222/token?format=markdown", "text", "markdown"},
		{"multi-unsupported-override", "slack://T111/B222/token?format=html", "text", "text"},
		{"multi-default", "slack://T111/B222/token?format=html", "html", "markdown"},
		{"relay-source", "apprise://example.com/config?format=html", "markdown", "markdown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseURL(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			if got := resolveTargetFormat(parsed, tc.input); got != tc.want {
				t.Fatalf("format=%q want %q", got, tc.want)
			}
		})
	}
}

func TestTargetFormatPassthroughAndReusedURL(t *testing.T) {
	parsed, err := ParseURL("slack://T111/B222/token")
	if err != nil {
		t.Fatal(err)
	}
	const content = "*literal* &amp; <b>HTML</b>"
	if got, err := ConvertMessageFormatForTarget(parsed, content, ""); err != nil || got != content {
		t.Fatalf("passthrough=%q error=%v", got, err)
	}
	if _, err := ConvertMessageFormatForTarget(parsed, content, "text"); err != nil {
		t.Fatal(err)
	}
	if got := targetNotifyFormat(parsed); got != "text" {
		t.Fatalf("text selected %q", got)
	}
	if _, err := ConvertMessageFormatForTarget(parsed, content, "markdown"); err != nil {
		t.Fatal(err)
	}
	if got := targetNotifyFormat(parsed); got != "markdown" {
		t.Fatalf("next call retained %q", got)
	}
	if parsed.Query["format"] != "" {
		t.Fatal("resolution changed the configured override")
	}
}

func TestTextToMarkdownEscapesLiteralControls(t *testing.T) {
	got, err := ConvertMessageFormat(`*literal* &amp; & bare.`, "text", "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if want := `\*literal\* \&amp; & bare\.`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
