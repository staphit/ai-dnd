import { useState, type FormEvent } from 'react';
import { ArrowRight, CheckCircle } from '@phosphor-icons/react';
import { MagneticButton } from './MagneticButton';
import type { PlayerId } from '../types';

interface ActionComposerProps {
  player: PlayerId;
  name: string;
  className: string;
  pending?: string;
  disabled: boolean;
  partySize: number;
  onSubmit: (player: PlayerId, text: string) => void;
}

export function ActionComposer({ player, name, className, pending, disabled, partySize, onSubmit }: ActionComposerProps) {
  const [text, setText] = useState('');
  const [error, setError] = useState('');

  function submit(event: FormEvent) {
    event.preventDefault();
    const value = text.trim();
    if (value.length < 3) {
      setError('請用至少三個字描述行動。');
      return;
    }
    setError('');
    onSubmit(player, value);
    setText('');
  }

  return (
    <form onSubmit={submit} className={`composer ${pending ? 'composer-submitted' : ''}`}>
      <div className="composer-title">
        <div>
          <span>{name}</span>
          <small>{className}</small>
        </div>
        {pending && <CheckCircle size={19} weight="fill" />}
      </div>
      {pending ? (
        <p className="submitted-copy">{pending}</p>
      ) : (
        <div className="input-group">
          <label htmlFor={`${player}-action`}>這一刻，你要做什麼？</label>
          <textarea
            id={`${player}-action`}
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="觀察石縫、保護同伴，或直接推開祭壇……"
            maxLength={2000}
            disabled={disabled}
            aria-describedby={`${player}-helper ${player}-error`}
          />
          <div className="input-foot">
            <small id={`${player}-helper`}>全隊 {partySize} 位玩家提交後推進</small>
            <span>{text.length}/2000</span>
          </div>
          {error && <p id={`${player}-error`} className="form-error">{error}</p>}
        </div>
      )}
      <MagneticButton type="submit" disabled={disabled || Boolean(pending)} className="composer-button">
        <span>{pending ? (partySize === 1 ? '等待裁定' : '等待同伴') : '鎖定行動'}</span>
        {!pending && <ArrowRight size={17} />}
      </MagneticButton>
    </form>
  );
}
