package sandbox

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func storeZip(store *Store, dest string, src any) ([]string, error) {
	if strings.TrimSpace(dest) == "" {
		return nil, fmt.Errorf("informe o caminho do arquivo zip")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var names []string

	err := addZipSource(store, zw, src, &names)
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}

	if _, err := store.WriteBytes(dest, buf.Bytes()); err != nil {
		return nil, err
	}
	return names, nil
}

func addZipSource(store *Store, zw *zip.Writer, src any, names *[]string) error {
	switch s := src.(type) {
	case []any:
		for _, item := range s {
			if err := addZipPath(store, zw, strings.TrimSpace(luaValueString(item)), names); err != nil {
				return err
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(s))
		for k := range s {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			entry, err := cleanZipEntry(k)
			if err != nil {
				return err
			}
			hdr := &zip.FileHeader{Name: entry, Method: zip.Deflate}
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			if _, err := w.Write([]byte(luaValueString(s[k]))); err != nil {
				return err
			}
			*names = append(*names, entry)
		}
	case string:
		return addZipPath(store, zw, strings.TrimSpace(s), names)
	default:
		return fmt.Errorf("origem do zip inválida: esperado array de caminhos, objeto nome→conteúdo ou um caminho")
	}
	return nil
}

func addZipPath(store *Store, zw *zip.Writer, relPath string, names *[]string) error {
	if relPath == "" {
		return nil
	}
	full, err := store.resolve(relPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("arquivo não encontrado: %s", relPath)
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if info.IsDir() {
		entries, err := os.ReadDir(full)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if e.Type()&os.ModeSymlink != 0 {
				continue
			}
			if err := addZipPath(store, zw, filepath.Join(relPath, e.Name()), names); err != nil {
				return err
			}
		}
		return nil
	}

	data, err := store.Read(relPath)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	entry, err := cleanZipEntry(filepath.ToSlash(relPath))
	if err != nil {
		return err
	}
	hdr.Name = entry
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(data)); err != nil {
		return err
	}
	*names = append(*names, entry)
	return nil
}

func storeUnzip(store *Store, src, dest string) ([]string, error) {
	srcFull, err := store.resolve(src)
	if err != nil {
		return nil, err
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return nil, fmt.Errorf("informe o destino da extração")
	}

	zr, err := zip.OpenReader(srcFull)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var extracted []string
	for _, f := range zr.File {
		entry, err := cleanZipEntry(f.Name)
		if err != nil {
			return nil, err
		}
		relTarget := filepath.ToSlash(filepath.Join(dest, entry))

		if f.FileInfo().IsDir() {
			if _, err := store.Mkdir(relTarget); err != nil {
				return nil, err
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, rerr := io.ReadAll(io.LimitReader(rc, maxFileBytes+1))
		rc.Close()
		if rerr != nil {
			return nil, rerr
		}
		if len(data) > maxFileBytes {
			return nil, fmt.Errorf("entrada no zip %q excede %d bytes", entry, maxFileBytes)
		}
		if _, err := store.WriteBytes(relTarget, data); err != nil {
			return nil, err
		}
		extracted = append(extracted, relTarget)
	}
	return extracted, nil
}

func cleanZipEntry(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = filepath.ToSlash(name)
	name = strings.TrimLeft(name, "/")
	cleaned := filepath.ToSlash(filepath.Clean(name))
	if cleaned == "" || cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") ||
		filepath.IsAbs(filepath.FromSlash(cleaned)) {
		return "", fmt.Errorf("caminho inválido no zip: %q", name)
	}
	return cleaned, nil
}
