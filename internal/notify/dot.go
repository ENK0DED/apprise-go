package notify

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	dotTextURL  = "https://dot.mindreset.tech/api/open/text"
	dotImageURL = "https://dot.mindreset.tech/api/open/image"

	dotModeText  = "text"
	dotModeImage = "image"
)

type DotTarget struct {
	apikey       string
	deviceID     string
	mode         string
	refreshNow   bool
	signature    string
	icon         string
	imageData    string
	link         string
	border       int
	ditherType   string
	ditherKernel string
}

func NewDotTarget(target *ParsedURL) (*DotTarget, error) {
	apikey := strings.TrimSpace(target.User)
	if apikey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	deviceID := strings.TrimSpace(target.Host)
	if deviceID == "" {
		return nil, fmt.Errorf("missing device id")
	}

	mode := dotModeText
	pathTokens := splitPath(target.Path)
	if len(pathTokens) > 0 {
		candidate := strings.ToLower(pathTokens[0])
		if candidate == dotModeText || candidate == dotModeImage {
			mode = candidate
		}
	}

	refreshNow := parseBoolWithDefault(target.Query["refresh"], true)

	signature := strings.TrimSpace(target.Query["signature"])
	icon := strings.TrimSpace(target.Query["icon"])
	imageData := strings.TrimSpace(target.Query["image"])
	link := strings.TrimSpace(target.Query["link"])

	border := 0
	if rawBorder := strings.TrimSpace(target.Query["border"]); rawBorder != "" {
		if value, err := strconv.Atoi(rawBorder); err == nil {
			border = value
		}
	}

	ditherType := strings.TrimSpace(target.Query["dither_type"])
	if ditherType == "" {
		ditherType = "DIFFUSION"
	}

	ditherKernel := strings.TrimSpace(target.Query["dither_kernel"])
	if ditherKernel == "" {
		ditherKernel = "FLOYD_STEINBERG"
	}

	if mode == dotModeText && imageData != "" {
		imageData = ""
	}

	return &DotTarget{
		apikey:       apikey,
		deviceID:     deviceID,
		mode:         mode,
		refreshNow:   refreshNow,
		signature:    signature,
		icon:         icon,
		imageData:    imageData,
		link:         link,
		border:       border,
		ditherType:   ditherType,
		ditherKernel: ditherKernel,
	}, nil
}

func (d *DotTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	spec, err := d.buildRequest(body, title)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (d *DotTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := d.buildRequest(body, title)
	if err != nil {
		return err
	}

	_ = notifyType

	return SendRequest(spec)
}

func (d *DotTarget) buildRequest(body, title string) (RequestSpec, error) {
	payload := map[string]any{
		"refreshNow": d.refreshNow,
		"deviceId":   d.deviceID,
	}

	requestURL := dotTextURL
	if d.mode == dotModeImage {
		if d.imageData == "" {
			return RequestSpec{}, fmt.Errorf("missing image data")
		}

		payload["image"] = d.imageData
		if d.link != "" {
			payload["link"] = d.link
		}
		payload["border"] = d.border
		payload["ditherType"] = d.ditherType
		payload["ditherKernel"] = d.ditherKernel

		requestURL = dotImageURL
	} else {
		if title != "" {
			payload["title"] = title
		}
		if body != "" {
			payload["message"] = body
		}
		if d.signature != "" {
			payload["signature"] = d.signature
		}
		if d.icon != "" {
			payload["icon"] = d.icon
		}
		if d.link != "" {
			payload["link"] = d.link
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"Authorization": "Bearer " + d.apikey,
			"Content-Type":  "application/json",
			"User-Agent":    "Apprise",
		},
		Body: string(data),
	}, nil
}
