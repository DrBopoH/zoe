package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"

	"economic/pkg/source"
)

// ───────────── Архивы с выписками ─────────────
//
// Binance (и не только) отдаёт выписку архивом. Архив распознаётся по сигнатуре, а не по расширению: zip, gzip, bzip2, tar
// (и их сочетания tar.gz / tar.bz2). Внутри берутся только файлы выписок (.csv, .pdf), остальное (README, метаданные) пропускается.
// Партия принимается целиком, только если все её файлы разобрались и указывают на один источник; иначе — отказ без записи.
// 7z и rar стандартная библиотека не читает: честный отказ с просьбой распаковать вручную.

const (
	maxArchiveEntries = 100       // файлов выписок в одном архиве
	maxEntryBytes     = 32 << 20  // как у одиночного файла выписки
	maxArchiveBytes   = 256 << 20 // всё распакованное: защита от архива-бомбы
)

type archiveFile struct {
	Name string
	Raw  []byte
}

// archiveKind — вид архива по первым байтам; "" — не архив.
func archiveKind(raw []byte) string {
	switch {
	case bytes.HasPrefix(raw, []byte("PK\x03\x04")) || bytes.HasPrefix(raw, []byte("PK\x05\x06")):
		return "zip"
	case bytes.HasPrefix(raw, []byte{0x1f, 0x8b}):
		return "gzip"
	case bytes.HasPrefix(raw, []byte("BZh")):
		return "bzip2"
	case len(raw) > 262 && string(raw[257:262]) == "ustar":
		return "tar"
	case bytes.HasPrefix(raw, []byte("7z\xbc\xaf\x27\x1c")):
		return "7z"
	case bytes.HasPrefix(raw, []byte("Rar!\x1a\x07")):
		return "rar"
	}
	return ""
}

func archiveErr(format string, a ...any) error {
	return &importError{http.StatusBadRequest, fmt.Sprintf(format, a...)}
}

// isStatementName — файл выписки по расширению (в архиве лежат и служебные файлы).
func isStatementName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".csv") || strings.HasSuffix(n, ".pdf")
}

// readLimited читает не больше limit байт; больше — ошибка (в память не пускаем).
func readLimited(r io.Reader, limit int64, what string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, archiveErr("Не удалось распаковать %s: %v", what, err)
	}
	if int64(len(b)) > limit {
		return nil, archiveErr("%s слишком большой после распаковки (больше %d МБ)", what, limit>>20)
	}
	return b, nil
}

// unpackArchive достаёт из архива файлы выписок. Вложенные архивы не раскрываются.
func unpackArchive(name string, raw []byte) ([]archiveFile, error) {
	var out []archiveFile
	total := int64(0)
	add := func(entry string, data []byte) error {
		base := path.Base(strings.ReplaceAll(entry, "\\", "/"))
		if base == "." || base == "/" || strings.HasPrefix(base, ".") || strings.HasPrefix(entry, "__MACOSX/") || !isStatementName(base) {
			return nil
		}
		if len(out) >= maxArchiveEntries {
			return archiveErr("В архиве больше %d файлов выписок", maxArchiveEntries)
		}
		total += int64(len(data))
		if total > maxArchiveBytes {
			return archiveErr("Архив слишком большой после распаковки (больше %d МБ)", maxArchiveBytes>>20)
		}
		out = append(out, archiveFile{Name: base, Raw: data})
		return nil
	}

	kind := archiveKind(raw)
	// gzip и bzip2: один файл или tar внутри
	var inner []byte
	switch kind {
	case "zip":
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, archiveErr("Не удалось открыть zip: %v", err)
		}
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() || !isStatementName(zf.Name) {
				continue
			}
			if zf.UncompressedSize64 > maxEntryBytes {
				return nil, archiveErr("Файл «%s» в архиве больше %d МБ — это не выписка", zf.Name, maxEntryBytes>>20)
			}
			rc, err := zf.Open()
			if err != nil {
				return nil, archiveErr("Не удалось открыть «%s» в архиве: %v", zf.Name, err)
			}
			data, err := readLimited(rc, maxEntryBytes, "«"+zf.Name+"»")
			rc.Close()
			if err != nil {
				return nil, err
			}
			if err := add(zf.Name, data); err != nil {
				return nil, err
			}
		}
		return out, nil
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, archiveErr("Не удалось открыть gzip: %v", err)
		}
		inner, err = readLimited(gr, maxArchiveBytes, "Архив")
		if err != nil {
			return nil, err
		}
		if archiveKind(inner) != "tar" { // просто сжатый файл: имя без .gz
			if len(inner) > maxEntryBytes {
				return nil, archiveErr("Файл в архиве больше %d МБ — это не выписка", maxEntryBytes>>20)
			}
			if err := add(strings.TrimSuffix(path.Base(name), path.Ext(name)), inner); err != nil {
				return nil, err
			}
			return out, nil
		}
	case "bzip2":
		b, err := readLimited(bzip2.NewReader(bytes.NewReader(raw)), maxArchiveBytes, "Архив")
		if err != nil {
			return nil, err
		}
		inner = b
		if archiveKind(inner) != "tar" {
			if len(inner) > maxEntryBytes {
				return nil, archiveErr("Файл в архиве больше %d МБ — это не выписка", maxEntryBytes>>20)
			}
			if err := add(strings.TrimSuffix(path.Base(name), path.Ext(name)), inner); err != nil {
				return nil, err
			}
			return out, nil
		}
	case "tar":
		inner = raw
	case "7z", "rar":
		return nil, archiveErr("Формат %s не поддерживается: распакуй архив и загрузи выписку (.zip, .gz, .tar, .tgz, .bz2 читаются сами)", kind)
	default:
		return nil, archiveErr("Не архив")
	}

	tr := tar.NewReader(bytes.NewReader(inner))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, archiveErr("Не удалось прочитать tar: %v", err)
		}
		if h.Typeflag != tar.TypeReg || !isStatementName(h.Name) {
			continue
		}
		if h.Size > maxEntryBytes {
			return nil, archiveErr("Файл «%s» в архиве больше %d МБ — это не выписка", h.Name, maxEntryBytes>>20)
		}
		data, err := readLimited(tr, maxEntryBytes, "«"+h.Name+"»")
		if err != nil {
			return nil, err
		}
		if err := add(h.Name, data); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// sameSource: все файлы партии указывают на один источник — тот же счёт (а у выписок Binance ещё и тот же пользователь).
func sameSource(files []*source.File) error {
	acc, profile := files[0].AccountID, ""
	for _, f := range files {
		if f.AccountID != acc {
			return archiveErr("Файлы в архиве относятся к разным источникам («%s» и «%s»): партия не загружена. Загрузи их по отдельности", files[0].Name, f.Name)
		}
		if f.Profile != "" {
			if profile != "" && f.Profile != profile {
				return archiveErr("Выписки в архиве принадлежат разным профилям Binance (%s и %s): партия не загружена", profile, f.Profile)
			}
			profile = f.Profile
		}
	}
	return nil
}

// parseUpload разбирает загруженный файл: выписку — как раньше, архив — в партию выписок одного источника.
// archived=true, если это был архив (даже с одним файлом внутри).
func parseUpload(filename string, raw []byte) (files []*source.File, archived bool, err error) {
	if archiveKind(raw) == "" {
		f, err := parseSource(filename, raw)
		if err != nil {
			return nil, false, err
		}
		return []*source.File{f}, false, nil
	}
	entries, err := unpackArchive(filename, raw)
	if err != nil {
		return nil, true, err
	}
	if len(entries) == 0 {
		return nil, true, archiveErr("В архиве нет файлов выписок (.csv или .pdf)")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	for _, e := range entries {
		f, err := parseSource(e.Name, e.Raw)
		if err != nil {
			if he, ok := err.(*importError); ok {
				return nil, true, &importError{he.code, "«" + e.Name + "» в архиве: " + he.msg}
			}
			return nil, true, err
		}
		files = append(files, f)
	}
	if err := sameSource(files); err != nil {
		return nil, true, err
	}
	return files, true, nil
}
