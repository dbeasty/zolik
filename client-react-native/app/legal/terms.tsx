import { LegalDocumentScreen } from '@/src/components/LegalDocumentScreen';
import { useLocale } from '@/src/hooks/useLocale';

/**
 * `/legal/terms`. On web this is also the stable URL to hand anyone who asks
 * for the terms — an app store, a host, a player — so it should not move.
 */
export default function TermsScreen() {
  useLocale();
  return <LegalDocumentScreen id="terms" />;
}
