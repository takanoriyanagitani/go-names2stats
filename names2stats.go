package names2stats

import (
	"bufio"
	"encoding/asn1"
	"encoding/json"
	"io"
	"io/fs"
	"iter"
	"os"
	"time"
)

type FileType = asn1.Enumerated

const (
	FileTypeUnspecified FileType = 0
	FileTypeBlck        FileType = 104
	FileTypeChar        FileType = 103
	FileTypeFldr        FileType = 105
	FileTypeRglr        FileType = 100
	FileTypeSyml        FileType = 102
	FileTypePipe        FileType = 106
	FileTypeSock        FileType = 109
)

type FileTypeToString func(FileType) string

type FileTypeToStringMap map[FileType]string

var fileTypeToStringMap FileTypeToStringMap = map[FileType]string{
	FileTypeUnspecified: "UNKNOWN FILE TYPE",
	FileTypeBlck:        "block special",
	FileTypeChar:        "character special",
	FileTypeFldr:        "directory",
	FileTypeRglr:        "regular file",
	FileTypeSyml:        "symbolic link",
	FileTypePipe:        "FIFO",
	FileTypeSock:        "socket",
}

var FileTypeToStringMapDefault FileTypeToStringMap = fileTypeToStringMap

func (m FileTypeToStringMap) ToFileTypeToString() FileTypeToString {
	return func(typ FileType) string {
		val, found := m[typ]
		switch found {
		case true:
			return val
		default:
			return m[FileTypeUnspecified]
		}
	}
}

var FileTypeToStringDefault FileTypeToString = FileTypeToStringMapDefault.
	ToFileTypeToString()

type UnixtimeUs int64

func (u UnixtimeUs) ToTime() time.Time {
	return time.UnixMicro(int64(u))
}

type BasicStatJson struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified_time"`
	FileType string    `json:"file_type"`
}

type BasicStat struct {
	Path     string `asn1:"utf8"`
	Size     int64
	Modified UnixtimeUs
	FileType
}

func (b BasicStat) ToJsonObj(t2s FileTypeToString) BasicStatJson {
	return BasicStatJson{
		Path:     b.Path,
		Size:     b.Size,
		Modified: b.Modified.ToTime(),
		FileType: t2s(b.FileType),
	}
}

func (b BasicStat) WithFullPath(f string) BasicStat {
	b.Path = f
	return b
}

type FileMode struct{ fs.FileMode }

func (m FileMode) IsSymlink() bool {
	return 0 != (m.FileMode & fs.ModeSymlink)
}

func (m FileMode) IsNamedPipe() bool {
	return 0 != (m.FileMode & fs.ModeNamedPipe)
}

func (m FileMode) IsSocket() bool {
	return 0 != (m.FileMode & fs.ModeSocket)
}

func (m FileMode) IsCharDevice() bool {
	return 0 != (m.FileMode & fs.ModeCharDevice)
}

func (m FileMode) IsDevice() bool {
	return 0 != (m.FileMode & fs.ModeDevice)
}

func (m FileMode) IsBlockDevice() bool {
	var isDevice bool = m.IsDevice()
	var isChar bool = m.IsCharDevice()
	var isNotChar bool = !isChar
	return isDevice && isNotChar
}

func (m FileMode) ToFileType() FileType {
	switch {
	case m.FileMode.IsRegular():
		return FileTypeRglr
	case m.FileMode.IsDir():
		return FileTypeFldr
	case m.IsSymlink():
		return FileTypeSyml
	case m.IsNamedPipe():
		return FileTypePipe
	case m.IsSocket():
		return FileTypeSock
	case m.IsCharDevice():
		return FileTypeChar
	case m.IsBlockDevice():
		return FileTypeBlck
	default:
		return FileTypeUnspecified
	}
}

type FileInfo struct{ fs.FileInfo }

func (i FileInfo) ToBasicStat() BasicStat {
	return BasicStat{
		Path:     "",
		Size:     i.Size(),
		Modified: UnixtimeUs(i.ModTime().UnixMicro()),
		FileType: FileMode{FileMode: i.FileInfo.Mode()}.ToFileType(),
	}
}

type FilenameToBasicStat func(string) (BasicStat, error)

type Root struct{ *os.Root }

func (r Root) Close() error { return r.Root.Close() }

func (r Root) NameToInfo(fullpath string) (fs.FileInfo, error) {
	return r.Root.Stat(fullpath)
}

func (r Root) NameToBasicStat(fullpath string) (BasicStat, error) {
	var empty BasicStat

	fi, e := r.NameToInfo(fullpath)
	if nil != e {
		return empty, e
	}

	return FileInfo{fi}.ToBasicStat().WithFullPath(fullpath), nil
}

func (r Root) ToFilenameToBasicStat() FilenameToBasicStat {
	return r.NameToBasicStat
}

func (r Root) NamesToBasicStatsToStdout(
	names iter.Seq[string],
) error {
	return r.ToFilenameToBasicStat().NamesToBasicStatsToStdout(names)
}

type RootDirname string

func (d RootDirname) ToRoot() (*os.Root, error) {
	return os.OpenRoot(string(d))
}

func (d RootDirname) NamesToBasicStatsToStdout(
	names iter.Seq[string],
) error {
	rt, e := d.ToRoot()
	if nil != e {
		return e
	}
	defer rt.Close()
	return Root{rt}.NamesToBasicStatsToStdout(names)
}

func (i FilenameToBasicStat) NamesToBasicStats(
	names iter.Seq[string],
) iter.Seq2[BasicStat, error] {
	return func(yield func(BasicStat, error) bool) {
		for name := range names {
			s, e := i(name)
			if !yield(s, e) {
				return
			}
		}
	}
}

func (i FilenameToBasicStat) NamesToBasicStatsToStdout(
	names iter.Seq[string],
) error {
	var stats iter.Seq2[BasicStat, error] = i.NamesToBasicStats(names)
	return BasicStatsToStdoutDefault(stats)
}

type BasicStatIter iter.Seq2[BasicStat, error]

func (i BasicStatIter) Collect() ([]BasicStat, error) {
	var ret []BasicStat
	for s, e := range i {
		if nil != e {
			return nil, e
		}
		ret = append(ret, s)
	}
	return ret, nil
}

func (c FileTypeToString) BasicStatsToWriter(
	wtr io.Writer,
) func(iter.Seq2[BasicStat, error]) error {
	return func(stats iter.Seq2[BasicStat, error]) error {
		var bw *bufio.Writer = bufio.NewWriter(wtr)
		defer bw.Flush()

		var enc *json.Encoder = json.NewEncoder(bw)
		for s, e := range stats {
			if nil != e {
				return e
			}

			var j BasicStatJson = s.ToJsonObj(c)
			e := enc.Encode(j)
			if nil != e {
				return e
			}
		}

		return nil
	}
}

func (c FileTypeToString) BasicStatsToStdout(
	stats iter.Seq2[BasicStat, error],
) error {
	return c.BasicStatsToWriter(os.Stdout)(stats)
}

var BasicStatsToStdoutDefault func(
	iter.Seq2[BasicStat, error],
) error = FileTypeToStringDefault.BasicStatsToStdout

type BasicStats []BasicStat

func (b BasicStats) ToAsn1DerBytes() ([]byte, error) {
	return asn1.Marshal(b)
}

func ReaderToNames(rdr io.Reader) iter.Seq[string] {
	return func(yield func(string) bool) {
		var s *bufio.Scanner = bufio.NewScanner(rdr)
		for s.Scan() {
			var fullpath string = s.Text()
			if !yield(fullpath) {
				return
			}
		}
	}
}

func StdinToNames() iter.Seq[string] { return ReaderToNames(os.Stdin) }
