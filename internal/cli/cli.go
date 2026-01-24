package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/version"
)

const usageText = "" +
	"Usage:\n" +
	"   apprise [OPTIONS] [APPRISE_URL [APPRISE_URL2 [APPRISE_URL3]]]\n" +
	"   apprise storage [OPTIONS] [ACTION] [UID1 [UID2 [UID3]]]\n"

func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apprise", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		body             string
		title            string
		notificationType string
		inputFormat      string
		disableAsync     bool
		showVersion      bool
		showHelp         bool
	)

	fs.StringVar(&body, "body", "", "Specify the message body.")
	fs.StringVar(&body, "b", "", "Specify the message body.")
	fs.StringVar(&title, "title", "", "Specify the message title.")
	fs.StringVar(&title, "t", "", "Specify the message title.")
	fs.StringVar(&notificationType, "notification-type", string(notify.NotifyInfo), "Specify the message type.")
	fs.StringVar(&notificationType, "n", string(notify.NotifyInfo), "Specify the message type.")
	fs.StringVar(&inputFormat, "input-format", "text", "Specify the message input format.")
	fs.StringVar(&inputFormat, "i", "text", "Specify the message input format.")
	fs.BoolVar(&disableAsync, "disable-async", false, "Send all notifications sequentially.")
	fs.BoolVar(&disableAsync, "Da", false, "Send all notifications sequentially.")
	fs.BoolVar(&showVersion, "version", false, "Display the apprise version and exit.")
	fs.BoolVar(&showVersion, "V", false, "Display the apprise version and exit.")
	fs.BoolVar(&showHelp, "help", false, "Show help.")
	fs.BoolVar(&showHelp, "h", false, "Show help.")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return 0
		}
		fmt.Fprintln(stderr, err)
		printUsage(stderr)
		return 2
	}

	if showHelp {
		printUsage(stdout)
		return 0
	}

	if showVersion {
		fmt.Fprintln(stdout, version.Message())
		return 0
	}

	if body == "" {
		data, err := io.ReadAll(os.Stdin)
		if err == nil {
			body = string(data)
		}
	}

	nt, ok := notify.ParseNotifyType(notificationType)
	if !ok {
		fmt.Fprintf(stderr, "unsupported notification type: %s\n", notificationType)
		return 2
	}

	urls := fs.Args()
	if len(urls) == 0 {
		printUsage(stdout)
		return 1
	}

	_ = inputFormat
	_ = disableAsync

	failed := false
	for _, rawURL := range urls {
		parsed, err := notify.ParseURL(rawURL)
		if err != nil {
			fmt.Fprintf(stderr, "invalid url: %s\n", err)
			failed = true
			continue
		}

		scheme := strings.ToLower(parsed.Scheme)
		switch scheme {
		case "json", "jsons":
			jsonTarget, err := notify.NewJSONTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "json target error: %s\n", err)
				failed = true
				continue
			}
			if err := jsonTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "json notify error: %s\n", err)
				failed = true
			}
		default:
			fmt.Fprintf(stderr, "unsupported url schema: %s\n", parsed.Scheme)
			failed = true
		}
	}

	if failed {
		return 1
	}

	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usageText)
}
