import "./HelpTip.css";

type HelpTipProps = {
  /** Подробный текст (можно с переносами строк) */
  text: string;
};

/** Кнопка «?» с всплывающей подсказкой при наведении и фокусе (клавиатура). */
export function HelpTip({ text }: HelpTipProps) {
  return (
    <span className="help-wrap">
      <button type="button" className="help-icon" aria-label={text}>
        ?
      </button>
      <span className="help-tooltip" role="tooltip">
        {text}
      </span>
    </span>
  );
}
