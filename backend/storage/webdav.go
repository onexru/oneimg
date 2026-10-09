package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type webDAVDriver struct {
	base       *url.URL
	user, pass string
	client     *http.Client
}

func NewWebDAV(base, user, pass string, client *http.Client) (Driver, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("invalid WebDAV endpoint")
	}
	return &webDAVDriver{u, user, pass, storageHTTPClient(client)}, nil
}
func storageHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: 30_000_000_000}
	}
	copy := *client
	// Never forward source credentials to a redirect target.
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}
func (d *webDAVDriver) request(ctx context.Context, method, key string, r io.Reader, size int64, contentType string) (*http.Response, error) {
	u := *d.base
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + key
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return nil, err
	}
	if d.user != "" {
		req.SetBasicAuth(d.user, d.pass)
	}
	if r != nil {
		req.ContentLength = size
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.New("WebDAV transport failed")
	}
	return resp, nil
}
func httpStorageError(status int) error {
	switch status {
	case 404:
		return ErrNotFound
	case 401, 403:
		return ErrAccessDenied
	default:
		return fmt.Errorf("storage HTTP status %d", status)
	}
}
func (d *webDAVDriver) Upload(ctx context.Context, ref Ref, r io.Reader, o UploadOptions) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	if o.Size < 0 {
		return Info{}, errors.New("unknown artifact size")
	}
	parent := path.Dir(key)
	if parent != "." {
		current := ""
		for _, part := range strings.Split(parent, "/") {
			current = path.Join(current, part)
			resp, e := d.request(ctx, "MKCOL", current, nil, 0, "")
			if e != nil {
				return Info{}, e
			}
			resp.Body.Close()
			if resp.StatusCode != 201 && resp.StatusCode != 204 && resp.StatusCode != 405 {
				return Info{}, httpStorageError(resp.StatusCode)
			}
		}
	}
	resp, err := d.request(ctx, http.MethodPut, key, r, o.Size, o.ContentType)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		return Info{}, httpStorageError(resp.StatusCode)
	}
	return Info{Size: o.Size, ContentType: o.ContentType}, nil
}
func (d *webDAVDriver) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return nil, err
	}
	resp, err := d.request(ctx, http.MethodGet, key, nil, 0, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, httpStorageError(resp.StatusCode)
	}
	return resp.Body, nil
}
func (d *webDAVDriver) Stat(ctx context.Context, ref Ref) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	resp, err := d.request(ctx, http.MethodHead, key, nil, 0, "")
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Info{}, httpStorageError(resp.StatusCode)
	}
	size, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	modified, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	return Info{Size: size, ContentType: resp.Header.Get("Content-Type"), Modified: modified}, nil
}
func (d *webDAVDriver) Delete(ctx context.Context, ref Ref) error {
	key, err := Key(ref.Key)
	if err != nil {
		return err
	}
	resp, err := d.request(ctx, http.MethodDelete, key, nil, 0, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 200 || resp.StatusCode == 204 {
		return nil
	}
	return httpStorageError(resp.StatusCode)
}
