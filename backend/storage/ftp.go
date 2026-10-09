package storage

import (
	"context"
	"errors"
	"github.com/jlaffaye/ftp"
	"io"
	"net"
	"net/textproto"
	"oneimg/backend/models"
	"path"
	"strconv"
	"strings"
	"time"
)

type ftpDriver struct{ config models.FTPBucket }

func NewFTP(c models.FTPBucket) Driver { return &ftpDriver{c} }

type ftpSession struct {
	conn  *ftp.ServerConn
	raw   net.Conn
	stops []func() bool
}

func (d *ftpDriver) connect(ctx context.Context) (*ftpSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	port := d.config.FTPPort
	if port == 0 {
		port = 21
	}
	session := &ftpSession{}
	dial := func(network, address string) (net.Conn, error) {
		c, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		deadline := time.Now().Add(2 * time.Minute)
		if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
			deadline = limit
		}
		c.SetDeadline(deadline)
		if session.raw == nil {
			session.raw = c
		}
		// Cancel control and passive data sockets alike; a blocked STOR write
		// must not wait for its full transfer timeout after client disconnect.
		session.stops = append(session.stops, context.AfterFunc(ctx, func() { c.Close() }))
		return c, nil
	}
	c, err := ftp.Dial(net.JoinHostPort(d.config.FTPHost, strconv.Itoa(port)), ftp.DialWithDialFunc(dial), ftp.DialWithContext(ctx))
	if err != nil {
		session.close()
		return nil, err
	}
	session.conn = c
	if err = c.Login(d.config.FTPUser, d.config.FTPPass); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}
func (s *ftpSession) close() error {
	for _, stop := range s.stops {
		stop()
	}
	if s.conn != nil {
		return s.conn.Quit()
	}
	if s.raw != nil {
		return s.raw.Close()
	}
	return nil
}
func (d *ftpDriver) Upload(ctx context.Context, ref Ref, r io.Reader, o UploadOptions) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	if o.Size < 0 {
		return Info{}, errors.New("unknown artifact size")
	}
	s, err := d.connect(ctx)
	if err != nil {
		return Info{}, err
	}
	defer s.close()
	parent := path.Dir(key)
	current := ""
	if parent != "." {
		for _, part := range strings.Split(parent, "/") {
			current = path.Join(current, part)
			if e := s.conn.MakeDir(current); e != nil {
				// MKD may report an existing directory as generic 550. Verify
				// it is traversable rather than treating every 550 as success.
				cwd, pwdErr := s.conn.CurrentDir()
				if pwdErr != nil {
					return Info{}, pwdErr
				}
				if dirErr := s.conn.ChangeDir(current); dirErr != nil {
					return Info{}, e
				}
				if dirErr := s.conn.ChangeDir(cwd); dirErr != nil {
					return Info{}, dirErr
				}
			}
		}
	}
	// Verify exactly the declared bytes were consumed. No whole-file buffering.
	reader := &countReader{r: &contextReader{ctx, r}}
	err = s.conn.Stor(key, io.LimitReader(reader, o.Size+1))
	if err != nil {
		return Info{}, ftpError(err)
	}
	if reader.n != o.Size {
		return Info{}, errors.New("stored artifact size mismatch")
	}
	return Info{Size: reader.n, ContentType: o.ContentType}, nil
}

type countReader struct {
	r io.Reader
	n int64
}

func (r *countReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}
func (d *ftpDriver) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return nil, err
	}
	s, err := d.connect(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.conn.Retr(key)
	if err != nil {
		s.close()
		return nil, ftpError(err)
	}
	deadline := time.Now().Add(2 * time.Minute)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	resp.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { resp.SetDeadline(time.Now()) })
	return &ftpReader{resp, s, stop}, nil
}

type ftpReader struct {
	r    *ftp.Response
	s    *ftpSession
	stop func() bool
}

func (r *ftpReader) Read(p []byte) (int, error) { return r.r.Read(p) }
func (r *ftpReader) Close() error               { r.stop(); err := r.r.Close(); return errors.Join(err, r.s.close()) }
func (d *ftpDriver) Stat(ctx context.Context, ref Ref) (Info, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return Info{}, err
	}
	s, err := d.connect(ctx)
	if err != nil {
		return Info{}, err
	}
	defer s.close()
	size, err := s.conn.FileSize(key)
	return Info{Size: size}, ftpError(err)
}
func (d *ftpDriver) Delete(ctx context.Context, ref Ref) error {
	key, err := Key(ref.Key)
	if err != nil {
		return err
	}
	s, err := d.connect(ctx)
	if err != nil {
		return err
	}
	defer s.close()
	err = ftpError(s.conn.Delete(key))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
func ftpError(err error) error {
	var e *textproto.Error
	if errors.As(err, &e) && e.Code == 550 {
		msg := strings.ToLower(e.Msg)
		if strings.Contains(msg, "not found") || strings.Contains(msg, "no such file") || strings.Contains(msg, "does not exist") {
			return ErrNotFound
		}
		// 550 also means permission denied; never release quota for it alone.
		if strings.Contains(msg, "permission") || strings.Contains(msg, "denied") {
			return ErrAccessDenied
		}
	}
	return err
}
