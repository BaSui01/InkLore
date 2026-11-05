package adapters_test

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/files"
	"bytes"
	"testing"
	"time"
)

func TestRenderedNovelFormatsAreStoredWithoutChangingBytes(t *testing.T) {
	novel := d.ExportNovel{ID: "e4a53d31-62a4-4a53-a748-0f4792250001", Title: "星海与归途", AuthorName: "作者"}
	volumes := []d.ExportVolume{{ID: "volume", Title: "第一卷"}}
	chapters := []d.ExportChapter{{ID: "chapter", VolumeID: "volume", Title: "第一章 & 启程", Body: "风吹过山谷。\nThis is a real chapter. <正文>"}}
	for _, format := range []string{d.JobFormatTXT, d.JobFormatMarkdown, d.JobFormatDOCX, d.JobFormatEPUB} {
		t.Run(format, func(t *testing.T) {
			rendered, err := d.RenderExport(format, novel, volumes, chapters, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := files.PrepareOutput(bytes.NewReader(rendered.Payload), rendered.Filename, rendered.MediaType, "export", &novel.ID)
			if err != nil {
				t.Fatalf("generated %s is rejected by file storage: %v", format, err)
			}
			if prepared.Descriptor().SizeBytes != int64(len(rendered.Payload)) || prepared.Descriptor().SHA256 == "" {
				t.Fatal("export bytes or digest missing")
			}
			if format == d.JobFormatMarkdown || format == d.JobFormatEPUB {
				if _, err := files.PrepareUpload(bytes.NewReader(rendered.Payload), rendered.Filename, rendered.MediaType, "import", &novel.ID); err == nil {
					t.Fatal("output format unexpectedly accepted as an upload")
				}
			}
		})
	}
}

func TestEPUBRejectsInvalidContainer(t *testing.T) {
	if _, err := files.PrepareOutput(bytes.NewReader([]byte("not a publication")), "novel.epub", "application/epub+zip", "export", nil); err == nil {
		t.Fatal("invalid EPUB was accepted")
	}
}
