import { useEffect, useRef, useState } from 'react';

export interface DiceAnimationState {
  rolling: boolean;
  outcome: 'success' | 'fail' | null;
}

export function useDiceAnimation() {
  const [diceAnimation, setDiceAnimation] = useState<DiceAnimationState>({
    rolling: false,
    outcome: null,
  });
  const timerRef = useRef<number | null>(null);

  useEffect(() => () => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
  }, []);

  function playDiceAnimation(success: boolean) {
    setDiceAnimation({ rolling: true, outcome: success ? 'success' : 'fail' });
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      setDiceAnimation({ rolling: false, outcome: null });
    }, 2600);
  }

  return { diceAnimation, playDiceAnimation };
}
