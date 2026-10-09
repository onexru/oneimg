package controllers

import (
	"context"
	"github.com/coreos/go-oidc/v3/oidc"
	"io"
	"net/http"
	"time"
)

type limitedResponseTransport struct {
	base  http.RoundTripper
	limit int64
}

func (transport limitedResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	response.Body = &limitedReadCloser{Reader: io.LimitReader(response.Body, transport.limit+1), Closer: response.Body}
	return response, nil
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
}

func externalHTTPClient(noRedirect bool, responseLimit int64) *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	} else {
		base = base.Clone()
	}
	base.ResponseHeaderTimeout = 10 * time.Second
	base.TLSHandshakeTimeout = 10 * time.Second
	base.MaxResponseHeaderBytes = 1 << 20
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: limitedResponseTransport{base: base, limit: responseLimit},
	}
	if noRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	}
	return client
}

func externalOIDCContext(ctx context.Context) context.Context {
	// Discovery、Token 和 JWKS 端点都应提供最终 URL；禁止 30x 避免 code/secret 被转发。
	return oidc.ClientContext(ctx, externalHTTPClient(true, externalAuthMaxHTTPBody))
}
