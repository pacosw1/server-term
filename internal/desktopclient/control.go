package desktopclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

func postControl(ctx context.Context, desktop config.Desktop, token, path string, query url.Values) error {
	base, tunnel, err := endpoint(ctx, desktop)
	if err != nil {
		return err
	}
	defer tunnel.Close()
	u := base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Servterm-Confirm", "yes")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("desktop control: %s", resp.Status)
	}
	return nil
}
func SendKey(ctx context.Context, desktop config.Desktop, token, combo string) error {
	q := url.Values{"combo": {combo}}
	return postControl(ctx, desktop, token, "/v1/key", q)
}
func Click(ctx context.Context, desktop config.Desktop, token string, x, y int) error {
	q := url.Values{"x": {strconv.Itoa(x)}, "y": {strconv.Itoa(y)}}
	return postControl(ctx, desktop, token, "/v1/click", q)
}
