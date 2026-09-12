import './app.css';
import App from '$product-app';
import { initI18n } from './lib/i18n';

// O idioma é definido antes da montagem para evitar um primeiro frame com
// `lang` incorreto e para permitir que componentes traduzíveis usem o catálogo
// desde o primeiro render.
initI18n();

const app = new App({
  target: document.getElementById('app')!,
});

export default app;
