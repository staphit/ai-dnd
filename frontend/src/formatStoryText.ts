/**
 * Story prose formatting done on the client — never rely on the DM model
 * for line breaks. Inserts breaks around spoken lines and cleans legacy junk.
 */

/** Remove legacy choice footers accidentally baked into narration. */
export function stripChoiceFooter(text: string): string {
  return text
    .replace(/\n\n可考慮：[^\n]*/g, '')
    .replace(/\n可考慮：[^\n]*/g, '')
    .replace(/可考慮：\s*[A-Za-z]{1,12}\s*$/g, '')
    .trim();
}

/**
 * Put each spoken line on its own visual block:
 * - before 「 … 」 dialogue
 * - after closing 」 (and trailing punctuation)
 * - before common speech tags like 說道／低語／喃喃
 */
export function formatDialogueBreaks(text: string): string {
  let t = text.replace(/\r\n/g, '\n').trim();
  if (!t) return t;

  // Opening quote mid-flow → new line
  t = t.replace(/([^\n「])(「)/g, '$1\n$2');
  // Closing quote then more prose → new line after quote block
  t = t.replace(/(」[。！？…—]*)(?=[^\n」])/g, '$1\n');
  // Speech verbs just before quote already handled; also break after quote+verb patterns
  // e.g. 「……。」托文補了一句 → already newline after 」
  // Before Latin/English style quotes used as dialogue (rare)
  t = t.replace(/([^\n"])("[\u4e00-\u9fff])/g, '$1\n$2');

  // Soft break before spoken-action tags when they start a new beat mid-paragraph
  t = t.replace(/([。！？])([\u4e00-\u9fff]{1,12}(?:低聲|輕聲|喃喃|碎念|嘀咕|低語|咕噥|說道|問道|喝道|怒道|笑道|喊道|接著說|繼續說)[：「])/g, '$1\n$2');

  // Collapse 3+ newlines to blank line; trim trailing spaces per line
  t = t
    .split('\n')
    .map((line) => line.trimEnd())
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim();

  return t;
}

export function formatStoryText(text: string): string {
  return formatDialogueBreaks(stripChoiceFooter(text));
}
