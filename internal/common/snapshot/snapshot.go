package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Pack(source, out string) (err error) {
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source must be a directory: %s", source)
	}

	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	defer func() {
		if closeErr := tw.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := gz.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	err = tarDir(source, out, tw)
	return err
}

func tarDir(source, skippedPath string, tw *tar.Writer) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		if samePath(path, skippedPath) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", path)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(rel)

		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}

		return nil
	})
}

func Unpack(src, dst string) (err error) {
	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := gz.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		target, err := archiveTargetPath(dst, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			out, err := os.OpenFile(
				target,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				os.FileMode(header.Mode),
			)
			if err != nil {
				return err
			}

			_, err = io.Copy(out, tr)
			closeErr := out.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}

		default:
			return fmt.Errorf("unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}

	return nil
}

func archiveTargetPath(dst, name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." {
		return dst, nil
	}
	if filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("archive entry resolves outside destination: %q", name)
	}
	if cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry resolves outside destination: %q", name)
	}

	target := filepath.Join(dst, cleanName)
	rel, err := filepath.Rel(dst, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry resolves outside destination: %q", name)
	}

	return target, nil
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}

	return filepath.Clean(a) == filepath.Clean(b)
}
