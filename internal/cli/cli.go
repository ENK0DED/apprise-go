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

	if body == "" {
		data, err := io.ReadAll(os.Stdin)
		if err == nil {
			body = string(data)
		}
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
		case "apprise", "apprises":
			appriseTarget, err := notify.NewAppriseTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "apprise target error: %s\n", err)
				failed = true
				continue
			}
			if err := appriseTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "apprise notify error: %s\n", err)
				failed = true
			}
		case "discord":
			discordTarget, err := notify.NewDiscordTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "discord target error: %s\n", err)
				failed = true
				continue
			}
			if err := discordTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "discord notify error: %s\n", err)
				failed = true
			}
		case "bark", "barks":
			barkTarget, err := notify.NewBarkTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "bark target error: %s\n", err)
				failed = true
				continue
			}
			if err := barkTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "bark notify error: %s\n", err)
				failed = true
			}
		case "freemobile":
			freeMobileTarget, err := notify.NewFreeMobileTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "freemobile target error: %s\n", err)
				failed = true
				continue
			}
			if err := freeMobileTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "freemobile notify error: %s\n", err)
				failed = true
			}
		case "gchat":
			gchatTarget, err := notify.NewGoogleChatTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "gchat target error: %s\n", err)
				failed = true
				continue
			}
			if err := gchatTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "gchat notify error: %s\n", err)
				failed = true
			}
		case "feishu":
			feishuTarget, err := notify.NewFeishuTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "feishu target error: %s\n", err)
				failed = true
				continue
			}
			if err := feishuTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "feishu notify error: %s\n", err)
				failed = true
			}
		case "lark":
			larkTarget, err := notify.NewLarkTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "lark target error: %s\n", err)
				failed = true
				continue
			}
			if err := larkTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "lark notify error: %s\n", err)
				failed = true
			}
		case "wxteams", "webex":
			webexTarget, err := notify.NewWebexTeamsTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "wxteams target error: %s\n", err)
				failed = true
				continue
			}
			if err := webexTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "wxteams notify error: %s\n", err)
				failed = true
			}
		case "line":
			lineTarget, err := notify.NewLineTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "line target error: %s\n", err)
				failed = true
				continue
			}
			if err := lineTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "line notify error: %s\n", err)
				failed = true
			}
		case "guilded":
			guildedTarget, err := notify.NewGuildedTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "guilded target error: %s\n", err)
				failed = true
				continue
			}
			if err := guildedTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "guilded notify error: %s\n", err)
				failed = true
			}
		case "dot":
			dotTarget, err := notify.NewDotTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "dot target error: %s\n", err)
				failed = true
				continue
			}
			if err := dotTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "dot notify error: %s\n", err)
				failed = true
			}
		case "splunk", "victorops":
			splunkTarget, err := notify.NewSplunkTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "splunk target error: %s\n", err)
				failed = true
				continue
			}
			if err := splunkTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "splunk notify error: %s\n", err)
				failed = true
			}
		case "workflow", "workflows":
			workflowTarget, err := notify.NewWorkflowsTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "workflow target error: %s\n", err)
				failed = true
				continue
			}
			if err := workflowTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "workflow notify error: %s\n", err)
				failed = true
			}
		case "flock":
			flockTarget, err := notify.NewFlockTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "flock target error: %s\n", err)
				failed = true
				continue
			}
			if err := flockTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "flock notify error: %s\n", err)
				failed = true
			}
		case "popcorn":
			popcornTarget, err := notify.NewPopcornTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "popcorn target error: %s\n", err)
				failed = true
				continue
			}
			if err := popcornTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "popcorn notify error: %s\n", err)
				failed = true
			}
		case "httpsms":
			httpSMSTarget, err := notify.NewHttpSMSTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "httpsms target error: %s\n", err)
				failed = true
				continue
			}
			if err := httpSMSTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "httpsms notify error: %s\n", err)
				failed = true
			}
		case "d7sms":
			d7Target, err := notify.NewD7NetworksTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "d7sms target error: %s\n", err)
				failed = true
				continue
			}
			if err := d7Target.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "d7sms notify error: %s\n", err)
				failed = true
			}
		case "atalk":
			atalkTarget, err := notify.NewAfricasTalkingTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "atalk target error: %s\n", err)
				failed = true
				continue
			}
			if err := atalkTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "atalk notify error: %s\n", err)
				failed = true
			}
		case "kavenegar":
			kavenegarTarget, err := notify.NewKavenegarTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "kavenegar target error: %s\n", err)
				failed = true
				continue
			}
			if err := kavenegarTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "kavenegar notify error: %s\n", err)
				failed = true
			}
		case "clickatell":
			clickatellTarget, err := notify.NewClickatellTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "clickatell target error: %s\n", err)
				failed = true
				continue
			}
			if err := clickatellTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "clickatell notify error: %s\n", err)
				failed = true
			}
		case "clicksend":
			clicksendTarget, err := notify.NewClickSendTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "clicksend target error: %s\n", err)
				failed = true
				continue
			}
			if err := clicksendTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "clicksend notify error: %s\n", err)
				failed = true
			}
		case "46elks", "elks":
			elksTarget, err := notify.NewFortySixElksTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "46elks target error: %s\n", err)
				failed = true
				continue
			}
			if err := elksTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "46elks notify error: %s\n", err)
				failed = true
			}
		case "seven":
			sevenTarget, err := notify.NewSevenTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "seven target error: %s\n", err)
				failed = true
				continue
			}
			if err := sevenTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "seven notify error: %s\n", err)
				failed = true
			}
		case "msgbird":
			messageBirdTarget, err := notify.NewMessageBirdTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "msgbird target error: %s\n", err)
				failed = true
				continue
			}
			if err := messageBirdTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "msgbird notify error: %s\n", err)
				failed = true
			}
		case "msg91":
			msg91Target, err := notify.NewMSG91Target(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "msg91 target error: %s\n", err)
				failed = true
				continue
			}
			if err := msg91Target.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "msg91 notify error: %s\n", err)
				failed = true
			}
		case "plivo":
			plivoTarget, err := notify.NewPlivoTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "plivo target error: %s\n", err)
				failed = true
				continue
			}
			if err := plivoTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "plivo notify error: %s\n", err)
				failed = true
			}
		case "vonage", "nexmo":
			vonageTarget, err := notify.NewVonageTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "vonage target error: %s\n", err)
				failed = true
				continue
			}
			if err := vonageTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "vonage notify error: %s\n", err)
				failed = true
			}
		case "twilio":
			twilioTarget, err := notify.NewTwilioTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "twilio target error: %s\n", err)
				failed = true
				continue
			}
			if err := twilioTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "twilio notify error: %s\n", err)
				failed = true
			}
		case "sns":
			snsTarget, err := notify.NewSNSTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "sns target error: %s\n", err)
				failed = true
				continue
			}
			if err := snsTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "sns notify error: %s\n", err)
				failed = true
			}
		case "bulkvs":
			bulkVSTarget, err := notify.NewBulkVSTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "bulkvs target error: %s\n", err)
				failed = true
				continue
			}
			if err := bulkVSTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "bulkvs notify error: %s\n", err)
				failed = true
			}
		case "bulksms":
			bulkSMSTarget, err := notify.NewBulkSMSTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "bulksms target error: %s\n", err)
				failed = true
				continue
			}
			if err := bulkSMSTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "bulksms notify error: %s\n", err)
				failed = true
			}
		case "burstsms":
			burstTarget, err := notify.NewBurstSMSTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "burstsms target error: %s\n", err)
				failed = true
				continue
			}
			if err := burstTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "burstsms notify error: %s\n", err)
				failed = true
			}
		case "smseagle", "smseagles":
			smseagleTarget, err := notify.NewSMSEagleTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "smseagle target error: %s\n", err)
				failed = true
				continue
			}
			if err := smseagleTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "smseagle notify error: %s\n", err)
				failed = true
			}
		case "smsmanager", "smsmgr":
			smsManagerTarget, err := notify.NewSMSManagerTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "smsmanager target error: %s\n", err)
				failed = true
				continue
			}
			if err := smsManagerTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "smsmanager notify error: %s\n", err)
				failed = true
			}
		case "sfr":
			sfrTarget, err := notify.NewSFRTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "sfr target error: %s\n", err)
				failed = true
				continue
			}
			if err := sfrTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "sfr notify error: %s\n", err)
				failed = true
			}
		case "voipms":
			voipmsTarget, err := notify.NewVoipmsTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "voipms target error: %s\n", err)
				failed = true
				continue
			}
			if err := voipmsTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "voipms notify error: %s\n", err)
				failed = true
			}
		case "sinch":
			sinchTarget, err := notify.NewSinchTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "sinch target error: %s\n", err)
				failed = true
				continue
			}
			if err := sinchTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "sinch notify error: %s\n", err)
				failed = true
			}
		case "signal", "signals":
			signalTarget, err := notify.NewSignalTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "signal target error: %s\n", err)
				failed = true
				continue
			}
			if err := signalTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "signal notify error: %s\n", err)
				failed = true
			}
		case "whatsapp":
			whatsAppTarget, err := notify.NewWhatsAppTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "whatsapp target error: %s\n", err)
				failed = true
				continue
			}
			if err := whatsAppTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "whatsapp notify error: %s\n", err)
				failed = true
			}
		case "rocket", "rockets":
			rocketTarget, err := notify.NewRocketChatTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "rocket target error: %s\n", err)
				failed = true
				continue
			}
			if err := rocketTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "rocket notify error: %s\n", err)
				failed = true
			}
		case "slack":
			slackTarget, err := notify.NewSlackTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "slack target error: %s\n", err)
				failed = true
				continue
			}
			if err := slackTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "slack notify error: %s\n", err)
				failed = true
			}
		case "revolt":
			revoltTarget, err := notify.NewRevoltTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "revolt target error: %s\n", err)
				failed = true
				continue
			}
			if err := revoltTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "revolt notify error: %s\n", err)
				failed = true
			}
		case "mmost", "mmosts":
			mmostTarget, err := notify.NewMattermostTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "mmost target error: %s\n", err)
				failed = true
				continue
			}
			if err := mmostTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "mmost notify error: %s\n", err)
				failed = true
			}
		case "dingtalk":
			dingTarget, err := notify.NewDingTalkTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "dingtalk target error: %s\n", err)
				failed = true
				continue
			}
			if err := dingTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "dingtalk notify error: %s\n", err)
				failed = true
			}
		case "nctalk", "nctalks":
			nctalkTarget, err := notify.NewNextcloudTalkTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "nctalk target error: %s\n", err)
				failed = true
				continue
			}
			if err := nctalkTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "nctalk notify error: %s\n", err)
				failed = true
			}
		case "threema":
			threemaTarget, err := notify.NewThreemaTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "threema target error: %s\n", err)
				failed = true
				continue
			}
			if err := threemaTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "threema notify error: %s\n", err)
				failed = true
			}
		case "tgram":
			tgramTarget, err := notify.NewTelegramTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "tgram target error: %s\n", err)
				failed = true
				continue
			}
			if err := tgramTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "tgram notify error: %s\n", err)
				failed = true
			}
		case "join":
			joinTarget, err := notify.NewJoinTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "join target error: %s\n", err)
				failed = true
				continue
			}
			if err := joinTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "join notify error: %s\n", err)
				failed = true
			}
		case "pagertree":
			pagertreeTarget, err := notify.NewPagerTreeTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pagertree target error: %s\n", err)
				failed = true
				continue
			}
			if err := pagertreeTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pagertree notify error: %s\n", err)
				failed = true
			}
		case "pagerduty":
			pagerDutyTarget, err := notify.NewPagerDutyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pagerduty target error: %s\n", err)
				failed = true
				continue
			}
			if err := pagerDutyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pagerduty notify error: %s\n", err)
				failed = true
			}
		case "psafer", "psafers":
			pushSaferTarget, err := notify.NewPushSaferTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "psafer target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushSaferTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "psafer notify error: %s\n", err)
				failed = true
			}
		case "lametric", "lametrics":
			lametricTarget, err := notify.NewLametricTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "lametric target error: %s\n", err)
				failed = true
				continue
			}
			if err := lametricTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "lametric notify error: %s\n", err)
				failed = true
			}
		case "fcm":
			fcmTarget, err := notify.NewFCMTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "fcm target error: %s\n", err)
				failed = true
				continue
			}
			if err := fcmTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "fcm notify error: %s\n", err)
				failed = true
			}
		case "vapid":
			vapidTarget, err := notify.NewVapidTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "vapid target error: %s\n", err)
				failed = true
				continue
			}
			if err := vapidTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "vapid notify error: %s\n", err)
				failed = true
			}
		case "ncloud", "nclouds":
			ncloudTarget, err := notify.NewNextcloudTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "ncloud target error: %s\n", err)
				failed = true
				continue
			}
			if err := ncloudTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "ncloud notify error: %s\n", err)
				failed = true
			}
		case "mastodon", "mastodons", "toot", "toots":
			mastodonTarget, err := notify.NewMastodonTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "mastodon target error: %s\n", err)
				failed = true
				continue
			}
			if err := mastodonTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "mastodon notify error: %s\n", err)
				failed = true
			}
		case "misskey", "misskeys":
			misskeyTarget, err := notify.NewMisskeyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "misskey target error: %s\n", err)
				failed = true
				continue
			}
			if err := misskeyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "misskey notify error: %s\n", err)
				failed = true
			}
		case "bluesky", "bsky":
			blueskyTarget, err := notify.NewBlueskyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "bluesky target error: %s\n", err)
				failed = true
				continue
			}
			if err := blueskyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "bluesky notify error: %s\n", err)
				failed = true
			}
		case "reddit":
			redditTarget, err := notify.NewRedditTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "reddit target error: %s\n", err)
				failed = true
				continue
			}
			if err := redditTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "reddit notify error: %s\n", err)
				failed = true
			}
		case "tweet", "twitter", "x":
			twitterTarget, err := notify.NewTwitterTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "twitter target error: %s\n", err)
				failed = true
				continue
			}
			if err := twitterTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "twitter notify error: %s\n", err)
				failed = true
			}
		case "twist":
			twistTarget, err := notify.NewTwistTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "twist target error: %s\n", err)
				failed = true
				continue
			}
			if err := twistTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "twist notify error: %s\n", err)
				failed = true
			}
		case "matrix", "matrixs":
			matrixTarget, err := notify.NewMatrixTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "matrix target error: %s\n", err)
				failed = true
				continue
			}
			if err := matrixTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "matrix notify error: %s\n", err)
				failed = true
			}
		case "opsgenie":
			opsgenieTarget, err := notify.NewOpsgenieTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "opsgenie target error: %s\n", err)
				failed = true
				continue
			}
			if err := opsgenieTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "opsgenie notify error: %s\n", err)
				failed = true
			}
		case "msteams":
			msteamsTarget, err := notify.NewMSTeamsTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "msteams target error: %s\n", err)
				failed = true
				continue
			}
			if err := msteamsTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "msteams notify error: %s\n", err)
				failed = true
			}
		case "ryver":
			ryverTarget, err := notify.NewRyverTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "ryver target error: %s\n", err)
				failed = true
				continue
			}
			if err := ryverTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "ryver notify error: %s\n", err)
				failed = true
			}
		case "zulip":
			zulipTarget, err := notify.NewZulipTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "zulip target error: %s\n", err)
				failed = true
				continue
			}
			if err := zulipTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "zulip notify error: %s\n", err)
				failed = true
			}
		case "wecombot":
			wecomTarget, err := notify.NewWeComBotTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "wecombot target error: %s\n", err)
				failed = true
				continue
			}
			if err := wecomTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "wecombot notify error: %s\n", err)
				failed = true
			}
		case "chanify":
			chanifyTarget, err := notify.NewChanifyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "chanify target error: %s\n", err)
				failed = true
				continue
			}
			if err := chanifyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "chanify notify error: %s\n", err)
				failed = true
			}
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
		case "form", "forms":
			formTarget, err := notify.NewFormTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "form target error: %s\n", err)
				failed = true
				continue
			}
			if err := formTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "form notify error: %s\n", err)
				failed = true
			}
		case "xml", "xmls":
			xmlTarget, err := notify.NewXMLTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "xml target error: %s\n", err)
				failed = true
				continue
			}
			if err := xmlTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "xml notify error: %s\n", err)
				failed = true
			}
		case "gotify", "gotifys":
			gotifyTarget, err := notify.NewGotifyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "gotify target error: %s\n", err)
				failed = true
				continue
			}
			if err := gotifyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "gotify notify error: %s\n", err)
				failed = true
			}
		case "ifttt":
			iftttTarget, err := notify.NewIFTTTTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "ifttt target error: %s\n", err)
				failed = true
				continue
			}
			if err := iftttTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "ifttt notify error: %s\n", err)
				failed = true
			}
		case "pushme":
			pushMeTarget, err := notify.NewPushMeTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushme target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushMeTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushme notify error: %s\n", err)
				failed = true
			}
		case "pushdeer", "pushdeers":
			pushDeerTarget, err := notify.NewPushDeerTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushdeer target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushDeerTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushdeer notify error: %s\n", err)
				failed = true
			}
		case "pushed":
			pushedTarget, err := notify.NewPushedTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushed target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushedTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushed notify error: %s\n", err)
				failed = true
			}
		case "pushy":
			pushyTarget, err := notify.NewPushyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushy target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushy notify error: %s\n", err)
				failed = true
			}
		case "pjet", "pjets":
			pushjetTarget, err := notify.NewPushjetTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushjet target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushjetTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushjet notify error: %s\n", err)
				failed = true
			}
		case "pbul":
			pushbulletTarget, err := notify.NewPushbulletTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushbullet target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushbulletTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushbullet notify error: %s\n", err)
				failed = true
			}
		case "push":
			pushTarget, err := notify.NewTechulusPushTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "push target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "push notify error: %s\n", err)
				failed = true
			}
		case "pover":
			pushoverTarget, err := notify.NewPushoverTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushover target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushoverTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushover notify error: %s\n", err)
				failed = true
			}
		case "prowl":
			prowlTarget, err := notify.NewProwlTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "prowl target error: %s\n", err)
				failed = true
				continue
			}
			if err := prowlTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "prowl notify error: %s\n", err)
				failed = true
			}
		case "qq":
			qqTarget, err := notify.NewQQTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "qq target error: %s\n", err)
				failed = true
				continue
			}
			if err := qqTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "qq notify error: %s\n", err)
				failed = true
			}
		case "notifico":
			notificoTarget, err := notify.NewNotificoTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "notifico target error: %s\n", err)
				failed = true
				continue
			}
			if err := notificoTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "notifico notify error: %s\n", err)
				failed = true
			}
		case "notica", "noticas":
			noticaTarget, err := notify.NewNoticaTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "notica target error: %s\n", err)
				failed = true
				continue
			}
			if err := noticaTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "notica notify error: %s\n", err)
				failed = true
			}
		case "spike":
			spikeTarget, err := notify.NewSpikeTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "spike target error: %s\n", err)
				failed = true
				continue
			}
			if err := spikeTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "spike notify error: %s\n", err)
				failed = true
			}
		case "signl4":
			signl4Target, err := notify.NewSignl4Target(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "signl4 target error: %s\n", err)
				failed = true
				continue
			}
			if err := signl4Target.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "signl4 notify error: %s\n", err)
				failed = true
			}
		case "strmlabs":
			streamlabsTarget, err := notify.NewStreamlabsTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "strmlabs target error: %s\n", err)
				failed = true
				continue
			}
			if err := streamlabsTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "strmlabs notify error: %s\n", err)
				failed = true
			}
		case "spugpush":
			spugpushTarget, err := notify.NewSpugpushTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "spugpush target error: %s\n", err)
				failed = true
				continue
			}
			if err := spugpushTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "spugpush notify error: %s\n", err)
				failed = true
			}
		case "spush":
			simplepushTarget, err := notify.NewSimplePushTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "spush target error: %s\n", err)
				failed = true
				continue
			}
			if err := simplepushTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "spush notify error: %s\n", err)
				failed = true
			}
		case "pushplus":
			pushplusTarget, err := notify.NewPushplusTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "pushplus target error: %s\n", err)
				failed = true
				continue
			}
			if err := pushplusTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "pushplus notify error: %s\n", err)
				failed = true
			}
		case "smtp2go":
			smtp2goTarget, err := notify.NewSMTP2GoTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "smtp2go target error: %s\n", err)
				failed = true
				continue
			}
			if err := smtp2goTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "smtp2go notify error: %s\n", err)
				failed = true
			}
		case "azure", "o365":
			office365Target, err := notify.NewOffice365Target(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "office365 target error: %s\n", err)
				failed = true
				continue
			}
			if err := office365Target.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "office365 notify error: %s\n", err)
				failed = true
			}
		case "sendpulse":
			sendPulseTarget, err := notify.NewSendPulseTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "sendpulse target error: %s\n", err)
				failed = true
				continue
			}
			if err := sendPulseTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "sendpulse notify error: %s\n", err)
				failed = true
			}
		case "sendgrid":
			sendgridTarget, err := notify.NewSendGridTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "sendgrid target error: %s\n", err)
				failed = true
				continue
			}
			if err := sendgridTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "sendgrid notify error: %s\n", err)
				failed = true
			}
		case "ses":
			sesTarget, err := notify.NewSESTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "ses target error: %s\n", err)
				failed = true
				continue
			}
			if err := sesTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "ses notify error: %s\n", err)
				failed = true
			}
		case "sparkpost":
			sparkpostTarget, err := notify.NewSparkPostTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "sparkpost target error: %s\n", err)
				failed = true
				continue
			}
			if err := sparkpostTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "sparkpost notify error: %s\n", err)
				failed = true
			}
		case "resend":
			resendTarget, err := notify.NewResendTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "resend target error: %s\n", err)
				failed = true
				continue
			}
			if err := resendTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "resend notify error: %s\n", err)
				failed = true
			}
		case "brevo":
			brevoTarget, err := notify.NewBrevoTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "brevo target error: %s\n", err)
				failed = true
				continue
			}
			if err := brevoTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "brevo notify error: %s\n", err)
				failed = true
			}
		case "mailgun":
			mailgunTarget, err := notify.NewMailgunTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "mailgun target error: %s\n", err)
				failed = true
				continue
			}
			if err := mailgunTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "mailgun notify error: %s\n", err)
				failed = true
			}
		case "ntfy", "ntfys":
			ntfyTarget, err := notify.NewNtfyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "ntfy target error: %s\n", err)
				failed = true
				continue
			}
			if err := ntfyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "ntfy notify error: %s\n", err)
				failed = true
			}
		case "schan":
			serverChanTarget, err := notify.NewServerChanTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "schan target error: %s\n", err)
				failed = true
				continue
			}
			if err := serverChanTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "schan notify error: %s\n", err)
				failed = true
			}
		case "dapnet":
			dapnetTarget, err := notify.NewDapnetTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "dapnet target error: %s\n", err)
				failed = true
				continue
			}
			if err := dapnetTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "dapnet notify error: %s\n", err)
				failed = true
			}
		case "enigma2", "enigma2s":
			enigmaTarget, err := notify.NewEnigma2Target(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "enigma2 target error: %s\n", err)
				failed = true
				continue
			}
			if err := enigmaTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "enigma2 notify error: %s\n", err)
				failed = true
			}
		case "emby", "embys":
			embyTarget, err := notify.NewEmbyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "emby target error: %s\n", err)
				failed = true
				continue
			}
			if err := embyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "emby notify error: %s\n", err)
				failed = true
			}
		case "hassio", "hassios":
			hassTarget, err := notify.NewHomeAssistantTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "hassio target error: %s\n", err)
				failed = true
				continue
			}
			if err := hassTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "hassio notify error: %s\n", err)
				failed = true
			}
		case "kumulos":
			kumulosTarget, err := notify.NewKumulosTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "kumulos target error: %s\n", err)
				failed = true
				continue
			}
			if err := kumulosTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "kumulos notify error: %s\n", err)
				failed = true
			}
		case "notifiarr":
			notifiarrTarget, err := notify.NewNotifiarrTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "notifiarr target error: %s\n", err)
				failed = true
				continue
			}
			if err := notifiarrTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "notifiarr notify error: %s\n", err)
				failed = true
			}
		case "napi", "notificationapi":
			notificationAPITarget, err := notify.NewNotificationAPITarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "notificationapi target error: %s\n", err)
				failed = true
				continue
			}
			if err := notificationAPITarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "notificationapi notify error: %s\n", err)
				failed = true
			}
		case "onesignal":
			oneSignalTarget, err := notify.NewOneSignalTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "onesignal target error: %s\n", err)
				failed = true
				continue
			}
			if err := oneSignalTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "onesignal notify error: %s\n", err)
				failed = true
			}
		case "parsep", "parseps":
			parseTarget, err := notify.NewParsePlatformTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "parsep target error: %s\n", err)
				failed = true
				continue
			}
			if err := parseTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "parsep notify error: %s\n", err)
				failed = true
			}
		case "synology", "synologys":
			synologyTarget, err := notify.NewSynologyTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "synology target error: %s\n", err)
				failed = true
				continue
			}
			if err := synologyTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "synology notify error: %s\n", err)
				failed = true
			}
		case "wxpusher":
			wxTarget, err := notify.NewWxPusherTarget(parsed)
			if err != nil {
				fmt.Fprintf(stderr, "wxpusher target error: %s\n", err)
				failed = true
				continue
			}
			if err := wxTarget.Send(body, title, nt); err != nil {
				fmt.Fprintf(stderr, "wxpusher notify error: %s\n", err)
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
