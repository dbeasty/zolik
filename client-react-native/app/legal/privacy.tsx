import { LegalDocumentScreen } from '@/src/components/LegalDocumentScreen';
import { useLocale } from '@/src/hooks/useLocale';

/**
 * `/legal/privacy`. Apple and Google both ask for a privacy-policy URL at
 * submission; this is it, and it is why the route is `/legal/privacy` rather
 * than something the router could renumber.
 */
export default function PrivacyScreen() {
  useLocale();
  return <LegalDocumentScreen id="privacy" />;
}
