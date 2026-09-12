type NavigationDirection = 'up' | 'down' | 'left' | 'right';

const focusableSelector = [
  '.tv-focusable:not([disabled])',
  'button:not([disabled])',
  'a[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"]):not([disabled])'
].join(',');

// Navegação espacial para a interface de dez pés. O foco DOM é a fonte de
// verdade; a classe visual é somente uma projeção, nunca estado paralelo.
export class SpatialNavigation {
  private static isInitialized = false;
  private static animationFrame = 0;
  private static pollTimer: number | null = null;
  private static isDocumentVisible = true;
  private static previousGamepadButtons = new Map<number, boolean[]>();

  public static init(): () => void {
    if (this.isInitialized) return () => this.destroy();
    this.isInitialized = true;
    this.isDocumentVisible = document.visibilityState !== 'hidden';
    window.addEventListener('keydown', this.handleKeyDown, true);
    document.addEventListener('visibilitychange', this.handleVisibilityChange);
    window.addEventListener('gamepadconnected', this.handleGamepadConnected);
    window.addEventListener('gamepaddisconnected', this.handleGamepadDisconnected);
    this.animationFrame = window.requestAnimationFrame(this.pollGamepads);
    return () => this.destroy();
  }

  public static destroy(): void {
    if (!this.isInitialized) return;
    window.removeEventListener('keydown', this.handleKeyDown, true);
    document.removeEventListener('visibilitychange', this.handleVisibilityChange);
    window.removeEventListener('gamepadconnected', this.handleGamepadConnected);
    window.removeEventListener('gamepaddisconnected', this.handleGamepadDisconnected);
    window.cancelAnimationFrame(this.animationFrame);
    if (this.pollTimer !== null) {
      window.clearTimeout(this.pollTimer);
      this.pollTimer = null;
    }
    this.previousGamepadButtons.clear();
    this.isDocumentVisible = true;
    this.isInitialized = false;
    document.querySelectorAll('.is-tv-focused').forEach((element) => element.classList.remove('is-tv-focused'));
  }

  public static focusFirst(): void {
    const [first] = this.visibleFocusables();
    if (first) this.setFocus(first);
  }

  private static handleKeyDown = (event: KeyboardEvent): void => {
    if (!document.documentElement.classList.contains('tv-mode') || event.defaultPrevented || event.isComposing || event.repeat) return;
    if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return;
    const target = event.target instanceof HTMLElement ? event.target : null;
    if (target?.closest('input, textarea, select, [contenteditable="true"], [role="menu"], input[type="range"], [role="slider"], video, audio')) return;
    // D-Pad no slider de seek/volume: delega ao controle nativo
    const inPlayerSeek = target?.closest('[data-component="player-seek-bar"]');
    const inPlayerVolume = target?.closest('[data-component="player-volume-bar"]');
    const directionByKey: Partial<Record<string, NavigationDirection>> = {
      ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right'
    };
    const direction = directionByKey[event.key];
    if (direction) {
      // Se foco no slider de seek/volume, delega à ação nativa
      if (direction === 'left' || direction === 'right') {
        if (inPlayerSeek || inPlayerVolume) {
          return; // deixa o slider nativo consumir Left/Right
        }
      }
      event.preventDefault();
      event.stopPropagation();
      this.navigate(direction);
      return;
    }
    if (event.key === 'Enter' && !event.repeat && target?.matches('.tv-focusable') && !target.closest('button, a[href], input, select, textarea')) {
      event.preventDefault();
      event.stopPropagation();
      target.click();
    }
  };

  private static isTextEntry(target: HTMLElement | null): boolean {
    return Boolean(target?.closest('input, textarea, select, [contenteditable="true"]'));
  }

  private static pressBack(): void {
    if (this.isTextEntry(document.activeElement instanceof HTMLElement ? document.activeElement : null)) return;
    const dialog = this.activeRegion();
    const closeButton = dialog?.querySelector<HTMLElement>('[data-action="close"]');
    if (closeButton) {
      closeButton.click();
      return;
    }
    window.history.back();
  }

  public static back(): void {
    this.pressBack();
  }

  private static handleVisibilityChange = (): void => {
    this.isDocumentVisible = document.visibilityState !== 'hidden';
    if (!this.isDocumentVisible) {
      this.previousGamepadButtons.clear();
    }
  };

  private static handleGamepadConnected = (event: GamepadEvent): void => {
    this.previousGamepadButtons.delete(event.gamepad.index);
  };

  private static handleGamepadDisconnected = (event: GamepadEvent): void => {
    this.previousGamepadButtons.delete(event.gamepad.index);
  };

  private static pollGamepads = (): void => {
    const tvModeEnabled = document.documentElement.classList.contains('tv-mode');
    if (this.isDocumentVisible && tvModeEnabled && navigator.getGamepads) {
      for (const gamepad of navigator.getGamepads()) {
        if (!gamepad) continue;
        const previous = this.previousGamepadButtons.get(gamepad.index) ?? [];
        const pressed = gamepad.buttons.map((button) => button.pressed);
        const onPress = (index: number, action: () => void): void => {
          if (pressed[index] && !previous[index]) action();
        };
        onPress(12, () => this.navigate('up'));
        onPress(13, () => this.navigate('down'));
        onPress(14, () => this.navigate('left'));
        onPress(15, () => this.navigate('right'));
        onPress(0, () => (document.activeElement as HTMLElement | null)?.click());
        onPress(1, () => this.pressBack());
        this.previousGamepadButtons.set(gamepad.index, pressed);
      }
    }
    // Não manter um loop de 60 Hz quando o documento está oculto ou fora do
    // modo TV. Um timer lento conserva a possibilidade de retomar sem custo
    // perceptível quando a janela volta a ser usada.
    if (this.isInitialized) {
      if (this.isDocumentVisible && tvModeEnabled) {
        this.animationFrame = window.requestAnimationFrame(this.pollGamepads);
      } else {
        this.pollTimer = window.setTimeout(() => {
          this.pollTimer = null;
          this.pollGamepads();
        }, 500);
      }
    }
  };

  private static navigate(direction: NavigationDirection): void {
    const focusables = this.visibleFocusables();
    if (focusables.length === 0) return;
    const current = document.activeElement instanceof HTMLElement && focusables.includes(document.activeElement)
      ? document.activeElement
      : null;
    if (!current) {
      this.setFocus(focusables[0]);
      return;
    }

    const currentRect = current.getBoundingClientRect();
    const currentX = currentRect.left + currentRect.width / 2;
    const currentY = currentRect.top + currentRect.height / 2;
    let best: { element: HTMLElement; score: number } | null = null;

    for (const candidate of focusables) {
      if (candidate === current) continue;
      const rect = candidate.getBoundingClientRect();
      const deltaX = rect.left + rect.width / 2 - currentX;
      const deltaY = rect.top + rect.height / 2 - currentY;
      const primary = direction === 'left' || direction === 'right' ? deltaX : deltaY;
      const secondary = direction === 'left' || direction === 'right' ? deltaY : deltaX;
      const pointsToDirection = direction === 'left' || direction === 'up' ? primary < -2 : primary > 2;
      if (!pointsToDirection) continue;
      const score = Math.abs(primary) + Math.abs(secondary) * 2.5;
      if (!best || score < best.score) best = { element: candidate, score };
    }
    if (best) this.setFocus(best.element);
  }

  private static visibleFocusables(): HTMLElement[] {
    const region = this.activeRegion() ?? document;
    return Array.from(region.querySelectorAll<HTMLElement>(focusableSelector)).filter((element) => {
      const style = window.getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0
        && !element.closest('[inert], [aria-hidden="true"]');
    });
  }

  private static activeRegion(): HTMLElement | null {
    const dialogs = Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]'))
      .filter((dialog) => dialog.getBoundingClientRect().width > 0);
    return dialogs.at(-1) ?? null;
  }

  private static setFocus(element: HTMLElement): void {
    document.querySelectorAll('.is-tv-focused').forEach((focused) => focused.classList.remove('is-tv-focused'));
    element.classList.add('is-tv-focused');
    element.focus({ preventScroll: true });
    element.scrollIntoView({
      behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
      block: 'nearest',
      inline: 'nearest'
    });
  }
}
