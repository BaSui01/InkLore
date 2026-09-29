/**
 * @inklore/ui - 统一认证弹窗 (AuthModal)
 * 纯净排版与零 Emoji 铁律，全面国际化
 * 复用统一鉴权组件 AuthView (variant="card")，保持 100% 行为与样式一致
 */

import React from "react";
import { Modal } from "./Modal";
import { AuthView, AuthViewProps, AuthMode, OAuthProvider } from "./AuthView";

export type { AuthMode, OAuthProvider };

export interface AuthModalProps extends Omit<AuthViewProps, "variant"> {
  open: boolean;
  onClose: () => void;
}

export function AuthModal({ open, onClose, ...props }: AuthModalProps) {
  return (
    <Modal open={open} onClose={onClose} maxWidth="md">
      <AuthView
        variant="card"
        {...props}
        onSuccess={(mode) => {
          props.onSuccess?.(mode);
          onClose();
        }}
      />
    </Modal>
  );
}
