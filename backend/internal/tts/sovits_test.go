package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(url string) *Client {
	return &Client{BaseURL: url, RefAudio: "/voices/narrator.wav", PromptText: "參考文本", PromptLang: "zh", TextLang: "zh"}
}

func TestSynthesizeSendsVoiceConfigAndReturnsAudio(t *testing.T) {
	wav := []byte("RIFFfakewav")
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tts" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "audio/wav")
		w.Write(wav)
	}))
	defer srv.Close()

	audio, mime, err := testClient(srv.URL).Synthesize(context.Background(), "隊伍推進。")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if string(audio) != string(wav) || mime != "audio/wav" {
		t.Errorf("audio/mime = %d bytes, %q", len(audio), mime)
	}
	if captured["text"] != "隊伍推進。" || captured["ref_audio_path"] != "/voices/narrator.wav" {
		t.Errorf("payload = %v", captured)
	}
	if captured["text_lang"] != "zh" || captured["prompt_text"] != "參考文本" {
		t.Errorf("voice config = %v", captured)
	}
}

func TestSynthesizeSurfacesServerMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "ref_audio_path not found"})
	}))
	defer srv.Close()

	_, _, err := testClient(srv.URL).Synthesize(context.Background(), "text")
	if err == nil || !strings.Contains(err.Error(), "ref_audio_path not found") {
		t.Errorf("expected server message, got %v", err)
	}
}

func TestSynthesizeRequiresVoiceConfig(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:9880"}
	_, _, err := c.Synthesize(context.Background(), "text")
	if err == nil || !strings.Contains(err.Error(), "SOVITS_REF_AUDIO") {
		t.Errorf("expected config error, got %v", err)
	}
}

func TestSynthesizeUnreachableGivesFriendlyError(t *testing.T) {
	c := testClient("http://127.0.0.1:1")
	_, _, err := c.Synthesize(context.Background(), "text")
	if err == nil || !strings.Contains(err.Error(), "無法連線本地 GPT-SoVITS") {
		t.Errorf("expected connection error, got %v", err)
	}
}

func TestPrepareTextDropsMetaAndSpeaksRules(t *testing.T) {
	in := "你推開石門，冷風湧出。守衛低喝：「站住！」（他按住劍柄）造成2d6傷害，武器+3，DC 15，HP 剩 7。\n\n檢定：甲 進行 DC 13 的力量（運動）檢定。祭壇沉重。\n\n可考慮：搜索祭壇／檢查泥痕"
	got := PrepareText(in)
	for _, banned := range []string{"檢定：甲", "可考慮", "／", "（", "）", "DC", "2d6", "HP", "+"} {
		if strings.Contains(got, banned) {
			t.Errorf("PrepareText left %q in %q", banned, got)
		}
	}
	for _, want := range []string{"2顆6面骰", "加3", "難度15", "生命值", "站住"} {
		if !strings.Contains(got, want) {
			t.Errorf("PrepareText missing %q in %q", want, got)
		}
	}
}

func TestPrepareTextTurnsNewlinesIntoSentenceBreaks(t *testing.T) {
	got := PrepareText("第一段\n第二段。\n\n第三段")
	if strings.Contains(got, "\n") {
		t.Errorf("newline survived: %q", got)
	}
	if got != "第一段。第二段。第三段" {
		t.Errorf("got %q", got)
	}
}

func TestPrepareTextRewritesBrackets(t *testing.T) {
	got := PrepareText("【行動駁回】理由如下")
	if got != "行動駁回：理由如下" {
		t.Errorf("got %q", got)
	}
}
