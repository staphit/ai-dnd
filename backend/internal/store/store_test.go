package store_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"dndduet/internal/store"
)

func open(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSaveAndGetImage(t *testing.T) {
	st := open(t)
	img := store.Image{
		Filename:  "1700000000000-abcd.png",
		Mime:      "image/png",
		Bytes:     []byte{0x89, 0x50, 0x4e, 0x47},
		Prompt:    "a prompt",
		Model:     "Codex $imagegen",
		CreatedAt: 1700000000000,
	}
	if err := st.SaveImage(img); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := st.GetImage(img.Filename)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Mime != img.Mime || !bytes.Equal(got.Bytes, img.Bytes) || got.CreatedAt != img.CreatedAt {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestGetMissingImage(t *testing.T) {
	st := open(t)
	_, ok, err := st.GetImage("nope.png")
	if err != nil || ok {
		t.Errorf("missing image: ok=%v err=%v", ok, err)
	}
}

func TestSaveImageRequiresFilename(t *testing.T) {
	st := open(t)
	if err := st.SaveImage(store.Image{Mime: "image/png", Bytes: []byte{1}}); err == nil {
		t.Error("expected error for empty filename")
	}
}

func TestConcurrentSaves(t *testing.T) {
	st := open(t)
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = st.SaveImage(store.Image{
				Filename:  fmt.Sprintf("file-%d.png", i),
				Mime:      "image/png",
				Bytes:     make([]byte, 200000),
				Prompt:    "p",
				Model:     "m",
				CreatedAt: int64(i),
			})
		}(i)
	}
	close(start)
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Errorf("concurrent save %d failed: %v", i, e)
		}
	}
}

func TestSaveImageReplaces(t *testing.T) {
	st := open(t)
	name := "dup.png"
	_ = st.SaveImage(store.Image{Filename: name, Mime: "image/png", Bytes: []byte{1}, Prompt: "a", Model: "m", CreatedAt: 1})
	if err := st.SaveImage(store.Image{Filename: name, Mime: "image/webp", Bytes: []byte{2, 3}, Prompt: "b", Model: "m", CreatedAt: 2}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _, _ := st.GetImage(name)
	if got.Mime != "image/webp" || len(got.Bytes) != 2 {
		t.Errorf("replace did not overwrite: %+v", got)
	}
}
