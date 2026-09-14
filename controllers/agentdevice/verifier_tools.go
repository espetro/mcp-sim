package agentdevice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/espetro/mcp-sim/pkg/contract"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tap taps the element with the given snapshot ref.
func (v *Verifier) Tap(ctx context.Context, target, ref string) error {
	_, err := v.call(ctx, "tap", withRef(deviceArgs(target), ref))
	return err
}

// DoubleTap double-taps the element with the given snapshot ref.
func (v *Verifier) DoubleTap(ctx context.Context, target, ref string) error {
	_, err := v.call(ctx, "double_tap", withRef(deviceArgs(target), ref))
	return err
}

// LongPress long-presses the element with the given snapshot ref.
func (v *Verifier) LongPress(ctx context.Context, target, ref string, ms int) error {
	args := withRef(deviceArgs(target), ref)
	args["duration_ms"] = ms
	_, err := v.call(ctx, "long_press", args)
	return err
}

// Swipe swipes the device screen in the given direction.
func (v *Verifier) Swipe(ctx context.Context, target, direction string) error {
	args := deviceArgs(target)
	args["direction"] = direction
	_, err := v.call(ctx, "swipe", args)
	return err
}

// Type enters text into the element with the given snapshot ref.
func (v *Verifier) Type(ctx context.Context, target, ref, text string) error {
	args := withRef(deviceArgs(target), ref)
	args["text"] = text
	_, err := v.call(ctx, "type", args)
	return err
}

// PressButton presses a hardware/system button.
func (v *Verifier) PressButton(ctx context.Context, target, button string) error {
	args := deviceArgs(target)
	args["button"] = button
	_, err := v.call(ctx, "press_button", args)
	return err
}

// OpenURL opens a URL or deep link on the device.
func (v *Verifier) OpenURL(ctx context.Context, target, url string) error {
	args := deviceArgs(target)
	args["url"] = url
	_, err := v.call(ctx, "open_url", args)
	return err
}

// Snapshot captures the current screen hierarchy.
func (v *Verifier) Snapshot(ctx context.Context, target string) (contract.Snapshot, error) {
	res, err := v.call(ctx, "snapshot", deviceArgs(target))
	if err != nil {
		return contract.Snapshot{}, err
	}
	snap, err := parseSnapshot(res)
	if err != nil {
		return contract.Snapshot{}, fmt.Errorf("agent-device: snapshot: %w", err)
	}
	snap.Target = target
	return snap, nil
}

// Screenshot captures the current screen as a base64 image.
func (v *Verifier) Screenshot(ctx context.Context, target string) (contract.Screenshot, error) {
	res, err := v.call(ctx, "screenshot", deviceArgs(target))
	if err != nil {
		return contract.Screenshot{}, err
	}
	shot, err := parseScreenshot(res)
	if err != nil {
		return contract.Screenshot{}, fmt.Errorf("agent-device: screenshot: %w", err)
	}
	shot.Target = target
	return shot, nil
}

// withRef adds the element ref to the argument set.
func withRef(args map[string]any, ref string) map[string]any {
	args["ref"] = ref
	return args
}

// toolErrorText extracts a message from an error CallToolResult.
func toolErrorText(res *sdkmcp.CallToolResult) string {
	for _, c := range res.Content {
		if t, ok := c.(*sdkmcp.TextContent); ok {
			return t.Text
		}
	}
	return "unknown error"
}

// structured returns the StructuredContent of a result, if any.
func structured(res *sdkmcp.CallToolResult) (map[string]any, bool) {
	if res == nil || res.StructuredContent == nil {
		return nil, false
	}
	m, ok := res.StructuredContent.(map[string]any)
	return m, ok
}

// parseSnapshot converts a snapshot tool result into a contract.Snapshot.
// Accepts both structured content and a JSON text content fallback.
func parseSnapshot(res *sdkmcp.CallToolResult) (contract.Snapshot, error) {
	var snap contract.Snapshot
	if m, ok := structured(res); ok {
		b, err := json.Marshal(m)
		if err == nil && json.Unmarshal(b, &snap) == nil {
			return snap, nil
		}
	}
	for _, c := range res.Content {
		t, ok := c.(*sdkmcp.TextContent)
		if !ok {
			continue
		}
		if err := json.Unmarshal([]byte(t.Text), &snap); err == nil {
			return snap, nil
		}
	}
	return contract.Snapshot{}, fmt.Errorf("no parsable snapshot in result (%d content blocks)", len(res.Content))
}

// parseScreenshot converts a screenshot tool result into a contract.Screenshot.
func parseScreenshot(res *sdkmcp.CallToolResult) (contract.Screenshot, error) {
	if m, ok := structured(res); ok {
		shot := contract.Screenshot{
			Format: str(m["format"]),
			Base64: str(m["base64"]),
		}
		if shot.Base64 != "" {
			return shot, nil
		}
	}
	for _, c := range res.Content {
		if img, ok := c.(*sdkmcp.ImageContent); ok {
			return contract.Screenshot{
				Format: imageFormat(img.MIMEType),
				Base64: base64.StdEncoding.EncodeToString(img.Data),
			}, nil
		}
	}
	return contract.Screenshot{}, fmt.Errorf("no image in result (%d content blocks)", len(res.Content))
}

// imageFormat maps a MIME type to a short format name.
func imageFormat(mime string) string {
	switch mime {
	case "image/jpeg":
		return "jpeg"
	default:
		return "png"
	}
}

// str coerces a decoded JSON value to a string.
func str(v any) string {
	s, _ := v.(string)
	return s
}
