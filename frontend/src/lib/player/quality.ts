import type { PlaybackPlan } from '../types';

export const AUTO_QUALITY_ID = 'auto';
export const AUDIO_ONLY_QUALITY_ID = 'audio-only';

export interface PlayerQualityOption {
  id: string;
  label: string;
  height: number;
  source: 'auto' | 'audio' | 'plan' | 'hls';
  requiresSeparateAudio: boolean;
}

export interface HLSQualityLevel {
  height?: number;
  name?: string;
}

export function buildPlanQualityOptions(plan: PlaybackPlan | null): PlayerQualityOption[] {
  const variants = (plan?.variants || [])
    .filter((variant) => Boolean(variant.id && variant.stream?.url))
    .map((variant) => ({
      id: `plan:${variant.id}`,
      label: variant.label || (variant.height ? `${variant.height}p` : 'Alternativa'),
      height: variant.height || 0,
      source: 'plan' as const,
      requiresSeparateAudio: variant.has_audio === false,
    }))
    .sort((first, second) => second.height - first.height);

  return withAutomaticQuality(variants, Boolean(plan?.audio_only?.url));
}

export function buildHLSQualityOptions(levels: HLSQualityLevel[], hasAudioOnly = false): PlayerQualityOption[] {
  const seenHeights = new Set<number>();
  const variants: PlayerQualityOption[] = [];

  levels.forEach((level, index) => {
    const height = level.height || 0;
    if (height > 0 && seenHeights.has(height)) return;
    if (height > 0) seenHeights.add(height);
    variants.push({
      id: `hls:${index}`,
      label: level.name || (height > 0 ? `${height}p` : `Nível ${index + 1}`),
      height,
      source: 'hls',
      requiresSeparateAudio: false,
    });
  });

  variants.sort((first, second) => second.height - first.height);
  return withAutomaticQuality(variants, hasAudioOnly);
}

export function planVariantForOption(plan: PlaybackPlan, optionID: string) {
  if (!optionID.startsWith('plan:')) return undefined;
  const variantID = optionID.slice('plan:'.length);
  return plan.variants?.find((variant) => variant.id === variantID && Boolean(variant.stream?.url));
}

function withAutomaticQuality(variants: PlayerQualityOption[], hasAudioOnly: boolean): PlayerQualityOption[] {
  // O seletor também é útil quando o resolvedor só encontrou o stream
  // principal: ele deixa explícito que estamos em modo automático e oferece
  // Somente áudio sempre que o plano possuir essa alternativa.
  const visibleVariants = variants;
  return [
    { id: AUTO_QUALITY_ID, label: 'Automática', height: 0, source: 'auto', requiresSeparateAudio: false },
    ...visibleVariants,
    ...(hasAudioOnly
      ? [{ id: AUDIO_ONLY_QUALITY_ID, label: 'Somente áudio', height: 0, source: 'audio' as const, requiresSeparateAudio: false }]
      : []),
  ];
}
