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
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "images"))
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
	if got.Mime != img.Mime || !bytes.Equal(got.Bytes, img.Bytes) {
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

func TestSaveImageRejectsTraversal(t *testing.T) {
	st := open(t)
	if err := st.SaveImage(store.Image{Filename: "../x.png", Mime: "image/png", Bytes: []byte{1}}); err == nil {
		t.Error("expected error for path traversal")
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
	if err := st.SaveImage(store.Image{Filename: name, Mime: "image/png", Bytes: []byte{2, 3}, Prompt: "b", Model: "m", CreatedAt: 2}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _, _ := st.GetImage(name)
	// Mime is derived from the file extension on disk.
	if got.Mime != "image/png" || !bytes.Equal(got.Bytes, []byte{2, 3}) {
		t.Errorf("replace did not overwrite: %+v", got)
	}
}

func TestDeleteImage(t *testing.T) {
	st := open(t)
	_ = st.SaveImage(store.Image{Filename: "a.png", Bytes: []byte{1}})
	_ = st.SaveImage(store.Image{Filename: "b.jpg", Bytes: []byte{2}})
	if err := st.DeleteImage("a.png"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := st.GetImage("a.png"); ok {
		t.Error("a.png should be gone")
	}
	if _, ok, _ := st.GetImage("b.jpg"); !ok {
		t.Error("b.jpg should remain")
	}
	// Idempotent.
	if err := st.DeleteImage("a.png"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestClearImages(t *testing.T) {
	st := open(t)
	_ = st.SaveImage(store.Image{Filename: "a.png", Bytes: []byte{1}})
	_ = st.SaveImage(store.Image{Filename: "b.jpg", Bytes: []byte{2}})
	if err := st.ClearImages(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, _ := st.GetImage("a.png"); ok {
		t.Error("a.png should be gone")
	}
	if _, ok, _ := st.GetImage("b.jpg"); ok {
		t.Error("b.jpg should be gone")
	}
}

func TestMemoryTextOnly(t *testing.T) {
	st := open(t)
	if err := st.AppendMemoryEvents("s1", []store.MemoryEvent{{Round: 1, Role: "dm", Text: "hello", CreatedAt: 1}}); err != nil {
		t.Fatal(err)
	}
	tail, err := st.MemoryTail("s1", 10)
	if err != nil || len(tail) != 1 || tail[0].Text != "hello" {
		t.Fatalf("memory: %+v err=%v", tail, err)
	}
}

func TestSceneSlotRoundTrip(t *testing.T) {
	st := open(t)
	slot := store.SceneSlot{
		ID: "slot-1", StoryID: "campaign-1", Scene: "chapel", Title: "test",
		Narration: "candles", ImagePrompt: "candlelit chapel", PlayersJSON: "[]", CreatedAt: 1,
	}
	if err := st.SaveSceneSlot(slot); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetSceneSlot("slot-1")
	if err != nil || !ok {
		t.Fatalf("get ok=%v err=%v", ok, err)
	}
	if got.ImagePrompt != "candlelit chapel" {
		t.Fatalf("got %+v", got)
	}
	if err := st.BindSceneSlotImage("slot-1", "/generated/x.png", "Grok"); err != nil {
		t.Fatal(err)
	}
	got, _, _ = st.GetSceneSlot("slot-1")
	if got.ImageURL != "/generated/x.png" {
		t.Fatalf("bound %+v", got)
	}
}
