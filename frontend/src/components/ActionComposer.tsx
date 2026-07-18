import { useState, type FormEvent } from 'react';
import { ArrowCounterClockwise, ArrowRight, CheckCircle } from '@phosphor-icons/react';
import { MagneticButton } from './MagneticButton';
import type { Choice, PlayerId } from '../types';
import { useI18n } from '../i18n';

interface ActionComposerProps {
  player: PlayerId;
  name: string;
  className: string;
  pending?: string;
  disabled: boolean;
  partySize: number;
  choices?: Choice[];
  resourceSummary?: string;
  /** Scripted campaign: no free-text input — clicking a choice locks it directly. */
  scripted?: boolean;
  combatActive?: boolean;
  onSubmit: (player: PlayerId, text: string) => void;
  onUnlock: (player: PlayerId) => void;
}

export function ActionComposer({ player, name, className, pending, disabled, partySize, choices = [], resourceSummary, scripted = false, combatActive = false, onSubmit, onUnlock }: ActionComposerProps) {
  const { lang, tz } = useI18n();
  const [text, setText] = useState('');
  const [error, setError] = useState('');

  function submit(event: FormEvent) {
    event.preventDefault();
    const value = text.trim();
    setError('');
    onSubmit(player, value || tz('本回合不行動，保持警戒。'));
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
      {combatActive ? (
        <div className="input-group scripted-composer">
          <p className="scripted-hint">{tz('戰鬥進行中：請在戰鬥面板行動')}</p>
        </div>
      ) : pending ? (
        <><p className="submitted-copy">{pending}</p><button type="button" className="unlock-action" disabled={disabled} onClick={() => onUnlock(player)}><ArrowCounterClockwise />{tz('修改行動')}</button></>
      ) : scripted && choices.length > 0 ? (
        <div className="input-group scripted-composer">
          <p className="scripted-hint" id={`${player}-helper`}>{tz('劇本模式：請從選項中選擇行動')}</p>
          <div className="action-choices" aria-label={tz('劇本選項')}>
            {choices.map((choice) => (
              <button type="button" key={choice.text} disabled={disabled} onClick={() => onSubmit(player, choice.text)}>{choice.text}</button>
            ))}
          </div>
          {resourceSummary && <p className="composer-resources">{tz('目前資源：')}{resourceSummary}</p>}
        </div>
      ) : (
        <div className="input-group">
          <label htmlFor={`${player}-action`}>{tz('這一刻，你要做什麼？')}</label>
          <textarea
            id={`${player}-action`}
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder={tz('觀察石縫、保護同伴，或直接推開祭壇……')}
            maxLength={2000}
            disabled={disabled}
            aria-describedby={`${player}-helper ${player}-error`}
          />
          {choices.length > 0 && <div className="action-choices" aria-label={tz('地城主建議選項')}>{choices.map((choice) => <button type="button" key={choice.text} onClick={() => setText(choice.text)}>{choice.text}</button>)}</div>}
          <div className="input-foot">
            <small id={`${player}-helper`}>{lang === 'en' ? `Leave blank to hold; the story advances once all ${partySize} players submit` : `可留白表示不行動；全隊 ${partySize} 位玩家提交後推進`}</small>
            <span>{text.length}/2000</span>
          </div>
          {resourceSummary && <p className="composer-resources">{tz('目前資源：')}{resourceSummary}</p>}
          {error && <p id={`${player}-error`} className="form-error">{error}</p>}
        </div>
      )}
      {(!scripted || Boolean(pending)) && (
        <MagneticButton type="submit" disabled={disabled || Boolean(pending)} className="composer-button">
          <span>{pending ? (partySize === 1 ? tz('等待裁定') : tz('等待同伴')) : tz('鎖定行動')}</span>
          {!pending && <ArrowRight size={17} />}
        </MagneticButton>
      )}
    </form>
  );
}
