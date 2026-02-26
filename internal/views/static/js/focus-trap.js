/**
 * Focus trap for modal dialogs.
 * Usage: <div x-data="focusTrap()" x-init="init($el)" @keydown.tab.prevent="handleTab($event)">
 *
 * Or simpler: call trapFocus(modalEl) on open, releaseFocus() on close.
 */

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function trapFocus(modal) {
    const focusable = [...modal.querySelectorAll(FOCUSABLE)];
    if (!focusable.length) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    modal._trapHandler = (e) => {
        if (e.key !== 'Tab') return;
        if (e.shiftKey) {
            if (document.activeElement === first) {
                e.preventDefault();
                last.focus();
            }
        } else {
            if (document.activeElement === last) {
                e.preventDefault();
                first.focus();
            }
        }
    };

    modal.addEventListener('keydown', modal._trapHandler);
    first.focus();
}

function releaseFocus(modal, returnTo) {
    if (modal._trapHandler) {
        modal.removeEventListener('keydown', modal._trapHandler);
        delete modal._trapHandler;
    }
    if (returnTo) returnTo.focus();
}
