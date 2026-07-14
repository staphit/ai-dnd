import { useState, type FormEvent } from 'react';
import { ArrowCounterClockwise, ArrowRight, CheckCircle } from '@phosphor-icons/react';
import { MagneticButton } from './MagneticButton';
import type { PlayerId } from '../types';

interface ActionComposerProps {
  player: PlayerId;
  name: string;
  className: string;
  pending?: string;
  disabled: boolean;
  partySize: number;
  choices?: string[];
  resourceSummary?: string;
  onSubmit: (player: PlayerId, text: string) => void;
  onUnlock: (player: PlayerId) => void;
}

export function ActionComposer({ player, name, className, pending, disabled, partySize, choices = [], resourceSummary, onSubmit, onUnlock }: ActionComposerProps) {
  const [text, setText] = useState('');
  const [error, setError] = useState('');

  function submit(event: FormEvent) {
    event.preventDefault();
    const value = text.trim();
    setError('');
    onSubmit(player, value || '本回合不行動，保持警戒。');
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
        <><p className="submitted-copy">{pending}</p><button type="button" className="unlock-action" disabled={disabled} onClick={() => onUnlock(player)}><ArrowCounterClockwise />修改行動</button></>
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
          {choices.length > 0 && <div className="action-choices" aria-label="地城主建議選項">{choices.map((choice) => <button type="button" key={choice} onClick={() => setText(choice)}>{choice}</button>)}</div>}
          <div className="input-foot">
            <small id={`${player}-helper`}>可留白表示不行動；全隊 {partySize} 位玩家提交後推進</small>
            <span>{text.length}/2000</span>
          </div>
          {resourceSummary && <p className="composer-resources">目前資源：{resourceSummary}</p>}
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
