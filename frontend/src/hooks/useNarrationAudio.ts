import { useEffect, useRef, useState } from 'react';
import { errorMessage } from '../app/app-utils';

export function useNarrationAudio(onNotice: (message: string) => void) {
  const [speaking, setSpeaking] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const blobUrlRef = useRef('');

  function releaseCurrent() {
    const audio = audioRef.current;
    if (audio) audio.pause();
    audioRef.current = null;
    if (blobUrlRef.current) URL.revokeObjectURL(blobUrlRef.current);
    blobUrlRef.current = '';
    setSpeaking(false);
  }

  useEffect(() => () => {
    const audio = audioRef.current;
    if (audio) audio.pause();
    if (blobUrlRef.current) URL.revokeObjectURL(blobUrlRef.current);
  }, []);

  async function speakNarration(text: string) {
    try {
      const response = await fetch('/api/tts', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ text }),
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || '語音合成失敗');
      }
      const url = URL.createObjectURL(await response.blob());
      releaseCurrent();
      blobUrlRef.current = url;
      const audio = new Audio(url);
      const stop = () => {
        if (blobUrlRef.current === url) {
          URL.revokeObjectURL(url);
          blobUrlRef.current = '';
        }
        setSpeaking(false);
      };
      audio.addEventListener('playing', () => setSpeaking(true));
      audio.addEventListener('ended', stop, { once: true });
      audio.addEventListener('error', stop, { once: true });
      audio.addEventListener('pause', () => setSpeaking(false));
      audioRef.current = audio;
      setSpeaking(true);
      void audio.play();
    } catch (caught) {
      setSpeaking(false);
      onNotice(`語音朗讀失敗：${errorMessage(caught)}`);
    }
  }

  return { speaking, speakNarration };
}
