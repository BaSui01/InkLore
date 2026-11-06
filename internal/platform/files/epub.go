package files

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"io/fs"
)

func validateEPUB(payload []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil || len(archive.File) < 3 || archive.File[0].Name != "mimetype" || archive.File[0].Method != zip.Store {
		return badType()
	}
	read := func(name string) ([]byte, error) {
		file, err := archive.Open(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(io.LimitReader(file, OutputLimit+1))
	}
	mediaType, err := read("mimetype")
	if err != nil || string(mediaType) != "application/epub+zip" {
		return badType()
	}
	container, err := read("META-INF/container.xml")
	if err != nil {
		return badType()
	}
	var manifest struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
			Type string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if xml.Unmarshal(container, &manifest) != nil || len(manifest.Rootfiles) != 1 {
		return badType()
	}
	root := manifest.Rootfiles[0]
	if !fs.ValidPath(root.Path) || root.Type != "application/oebps-package+xml" {
		return badType()
	}
	packageXML, err := read(root.Path)
	if err != nil {
		return badType()
	}
	var pkg struct {
		XMLName xml.Name `xml:"http://www.idpf.org/2007/opf package"`
		Version string   `xml:"version,attr"`
	}
	if xml.Unmarshal(packageXML, &pkg) != nil || pkg.Version != "3.0" {
		return badType()
	}
	return nil
}
