package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type localDriver struct{ root string }

func NewLocal(root string) Driver { return &localDriver{root: root} }
func (d *localDriver) resolve(ref Ref) (string, error) {
	key, err := Key(ref.Key)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(key, "uploads/") {
		return "", ErrInvalidKey
	}
	root, err := filepath.Abs(d.root)
	if err != nil {
		return "", err
	}
	// filepath.Join 直接以 root 为前缀构造，天然兼容 Unix 根与 Windows
	// 卷前缀（"D:\..."）。此前逐分隔符从 string(filepath.Separator)
	// 重建的做法在盘符路径下会得到 "\D:" 而非 "D:\"。
	full := filepath.Join(root, filepath.FromSlash(key))
	if rel, relErr := filepath.Rel(root, full); relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}
	// 逐组件 Lstat 拒绝符号链接（含 root 自身与 root 的各级祖先）。
	// Windows 盘符根从卷名起步，UNC 根保持 \\server\share 前缀完整。
	sep := string(filepath.Separator)
	var current string
	var parts []string
	if strings.HasPrefix(root, sep) && !strings.HasPrefix(root, `\\`) {
		current = sep
		parts = strings.Split(strings.TrimPrefix(root, sep), sep)
	} else {
		vol := filepath.VolumeName(root)
		current = vol
		parts = strings.Split(strings.TrimPrefix(root, vol), sep)
	}
	parts = append(parts, strings.Split(strings.TrimPrefix(key, "uploads/"), "/")...)
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", ErrInvalidKey
		}
	}
	return full, nil
}
func (d *localDriver) Upload(ctx context.Context, ref Ref, r io.Reader, o UploadOptions) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	if o.Size < 0 {
		return Info{}, ErrInvalidKey
	}
	name, err := d.resolve(ref)
	if err != nil {
		return Info{}, err
	}
	if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return Info{}, err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".oneimg-*")
	if err != nil {
		return Info{}, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(&contextReader{ctx, r}, o.Size+1))
	if err != nil {
		return Info{}, err
	}
	if n != o.Size {
		return Info{}, errors.New("stored artifact size mismatch")
	}
	mode := os.FileMode(0644)
	if o.Private {
		mode = 0600
	}
	if err = f.Chmod(mode); err != nil {
		return Info{}, err
	}
	if err = f.Close(); err != nil {
		return Info{}, err
	}
	if err = ctx.Err(); err != nil {
		return Info{}, err
	}
	if err = os.Rename(f.Name(), name); err != nil {
		return Info{}, err
	}
	return Info{Size: n, ContentType: o.ContentType}, nil
}
func (d *localDriver) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name, err := d.resolve(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, localError(err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		if err == nil {
			err = ErrAccessDenied
		}
		return nil, err
	}
	return f, nil
}
func (d *localDriver) Stat(ctx context.Context, ref Ref) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	name, err := d.resolve(ref)
	if err != nil {
		return Info{}, err
	}
	info, err := os.Stat(name)
	if err != nil {
		return Info{}, localError(err)
	}
	if !info.Mode().IsRegular() {
		return Info{}, ErrAccessDenied
	}
	return Info{Size: info.Size(), Modified: info.ModTime()}, nil
}
func (d *localDriver) Delete(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := d.resolve(ref)
	if err != nil {
		return err
	}
	info, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrAccessDenied
	}
	err = os.Remove(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func localError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		return ErrAccessDenied
	}
	return err
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
