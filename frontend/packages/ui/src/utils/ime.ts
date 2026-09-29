/**
 * @inklore/ui - 输入法（IME）状态识别与按键防误触工具
 * 遵循 Zero Hardcoding Contract 与 No Emoji Policy
 * 满足 RE08（IME 不提交半字）与 DS03（IME 不误触发快捷键）设计红线
 */

import React, { useRef, useCallback, useState } from "react";

/**
 * 判断键盘事件是否处于输入法组合（Composition）过程中
 * 综合检测 React 原生事件、标准 DOM 事件、虚拟按键码 229 以及 Process 键
 */
export function isImeComposing(event: React.KeyboardEvent | KeyboardEvent): boolean {
  // 优先取底层原生事件属性
  const native = "nativeEvent" in event ? event.nativeEvent : event;

  if (native && typeof (native as KeyboardEvent).isComposing === "boolean" && (native as KeyboardEvent).isComposing) {
    return true;
  }

  if (typeof (event as KeyboardEvent).isComposing === "boolean" && (event as KeyboardEvent).isComposing) {
    return true;
  }

  // keyCode 229 是各类浏览器与 IME 引擎在拼音/注音/五笔等选词中发出的固定键码
  if ("keyCode" in event && event.keyCode === 229) {
    return true;
  }

  // Windows / Chromium 下 IME 处理态 key 为 Process
  if (event.key === "Process") {
    return true;
  }

  return false;
}

export interface UseImeCompositionOptions {
  onCompositionStart?: (e: React.CompositionEvent) => void;
  onCompositionEnd?: (e: React.CompositionEvent) => void;
}

/**
 * 管理与追踪 IME 组合生命周期
 * 提供 50ms 缓冲标记，防止部分浏览器中选词确认的 Enter 键先于或紧随 compositionend 触发导致的误判
 */
export function useImeComposition(options?: UseImeCompositionOptions) {
  const [isComposing, setIsComposing] = useState(false);
  const isComposingRef = useRef(false);
  const justEndedRef = useRef(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleCompositionStart = useCallback((e: React.CompositionEvent) => {
    isComposingRef.current = true;
    setIsComposing(true);
    justEndedRef.current = false;
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    options?.onCompositionStart?.(e);
  }, [options]);

  const handleCompositionEnd = useCallback((e: React.CompositionEvent) => {
    isComposingRef.current = false;
    setIsComposing(false);
    justEndedRef.current = true;

    if (timerRef.current) {
      clearTimeout(timerRef.current);
    }
    // 50ms 缓冲，确保随后的 keyup / keydown 消费完毕
    timerRef.current = setTimeout(() => {
      justEndedRef.current = false;
      timerRef.current = null;
    }, 50);

    options?.onCompositionEnd?.(e);
  }, [options]);

  /**
   * 检查当前是否应拦截快捷按键动作
   */
  const shouldBlockKeyAction = useCallback((e: React.KeyboardEvent | KeyboardEvent) => {
    return isComposingRef.current || justEndedRef.current || isImeComposing(e);
  }, []);

  return {
    isComposing,
    isComposingRef,
    justEndedRef,
    handleCompositionStart,
    handleCompositionEnd,
    shouldBlockKeyAction,
  };
}

/**
 * 包装快捷键处理函数，确保在 IME 组合期间绝不触发
 */
export function createImeSafeKeyHandler<T extends React.KeyboardEvent | KeyboardEvent>(
  handler: (e: T) => void,
  isComposingRef?: React.RefObject<boolean>
) {
  return (e: T) => {
    if (isComposingRef?.current || isImeComposing(e)) {
      return;
    }
    handler(e);
  };
}

