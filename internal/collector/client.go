package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/PatrickStar12138/fn-disk-wakeup-tracker/internal/api"
)

// Client 通过固定内部 Unix Socket 向 Server 上报采集批次。
type Client struct {
	SocketPath string
	http       *http.Client
}

// NewClient 创建带连接和请求超时的 Unix HTTP 客户端。
func NewClient(socket string) *Client {
	d := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return d.DialContext(ctx, "unix", socket)
	}, DisableKeepAlives: false, MaxIdleConns: 1, IdleConnTimeout: 30 * time.Second}
	return &Client{SocketPath: socket, http: &http.Client{Transport: transport, Timeout: 5 * time.Second}}
}

// Send 发送有界 JSON 批次并要求 Server 明确返回成功状态。
func (c *Client) Send(ctx context.Context, p api.CollectorPayload) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/collect", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("collector endpoint status %d", res.StatusCode)
	}
	return nil
}
