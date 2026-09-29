import React, { useRef, useState } from "react";
import { Bot } from "lucide-react";
import { useTranslation } from "@inklore/i18n";
import { isImeComposing } from "@inklore/ui";

export interface PlainTextEditorProps {
  value: string;
  onChange: (value: string) => void;
  onSave: () => void;
  onOpenWorkbench: (selectedText: string, startUtf16: number, endUtf16: number) => void;
  fontSize: number;
  disabled?: boolean;
  placeholder?: string;
}

export function PlainTextEditor({ value, onChange, onSave, onOpenWorkbench, fontSize, disabled = false, placeholder }: PlainTextEditorProps) {
  const { t } = useTranslation();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const isComposingRef = useRef(false);
  const [selection, setSelection] = useState<{ text: string; start: number; end: number } | null>(null);

  const handleSelect = () => {
    const element = textareaRef.current;
    if (!element || element.selectionStart === element.selectionEnd) {
      setSelection(null);
      return;
    }
    const start = element.selectionStart;
    const end = element.selectionEnd;
    setSelection({ text: element.value.slice(start, end), start, end });
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((event.ctrlKey || event.metaKey) && ["s", "S"].includes(event.key)) {
      event.preventDefault();
      if (!isComposingRef.current && !isImeComposing(event)) onSave();
      return;
    }
    if (event.key === "Tab") {
      event.preventDefault();
      const element = textareaRef.current;
      if (!element) return;
      const start = element.selectionStart;
      const end = element.selectionEnd;
      onChange(value.slice(0, start) + "  " + value.slice(end));
      requestAnimationFrame(() => { element.selectionStart = element.selectionEnd = start + 2; });
    }
  };

  return (
    <div className="relative flex min-h-[500px] w-full flex-col">
      {selection && <div className="absolute z-40 flex items-center gap-2 rounded-xl border border-stone-700 bg-stone-900 px-2.5 py-1.5 text-xs font-medium text-white shadow-2xl">
        <span className="border-r border-stone-700 px-1 pr-2 font-mono text-[11px] text-stone-400">{t("agentWorkbench.selectionCount", Array.from(selection.text).length)}</span>
        <button type="button" onClick={() => onOpenWorkbench(selection.text, selection.start, selection.end)} className="flex items-center gap-1 rounded bg-amber-600 px-2 py-1 font-semibold hover:bg-amber-500"><Bot className="h-3 w-3" /><span>{t("agentWorkbench.open")}</span></button>
      </div>}
      <textarea ref={textareaRef} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} onSelect={handleSelect} onKeyDown={handleKeyDown}
        onCompositionStart={() => { isComposingRef.current = true; }} onCompositionEnd={() => { isComposingRef.current = false; }}
        placeholder={placeholder || t("agentWorkbench.editorPlaceholder")} style={{ fontSize: `${fontSize}px`, lineHeight: 1.85 }}
        className="novel-reading-font min-h-[550px] w-full flex-1 resize-none border-none bg-transparent p-4 tracking-wide text-stone-800 outline-none placeholder:text-stone-400 focus:outline-none selection:bg-amber-200 dark:text-stone-200 dark:placeholder:text-stone-600 dark:selection:bg-amber-900/60" />
    </div>
  );
}


