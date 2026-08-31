import { LegalDocumentScreen } from '@/src/components/LegalDocumentScreen';

/**
 * `/legal/terms`. On web this is also the stable URL to hand anyone who asks
 * for the terms — an app store, a host, a player — so it should not move.
 */
export default function TermsScreen() {
  return <LegalDocumentScreen id="terms" />;
}
